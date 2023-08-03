package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// https://discord.com/developers/docs/resources/channel#create-message-jsonform-params
type SendChannelMessageRequest struct {
	Content string `json:"content" validate:"required,max=2000"`
}

type ErrorResponse struct {
	Message string `json:"message"`
}

var apiBaseUrl string

/*
This is the regex for detecting the event in which the a tab is added to the chat window.
To detect if this was a player message tab DE prepend 'F' to the username so we can be certain
it was a player direct message.
*/
var r = regexp.MustCompile("(Script \\[Info\\]: ChatRedux\\.lua: ChatRedux::AddTab: Adding tab with channel name: F)(.+)( to index.+)")

var discordBearerToken = ""

func main() {
	fmt.Println("Starting!")

	// This is added because when we use 'go build' we inject a build-time variable into apiBaseUrl
	// so it won't be empty string, however, when using docker, we want to get this from the
	// environment variable
	if apiBaseUrl == "" {
		apiBaseUrl = os.Getenv("API_BASE_URL")
	}

	http.HandleFunc("/api/v1/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		discordBearerToken = r.URL.Query().Get("token")

		w.WriteHeader(200)
		w.Write([]byte("Successful! You may close this tab and navigate back to your command line."))
		return
	})

	go func() {
		log.Fatal(http.ListenAndServe(":8081", nil))
	}()

	eeLogPath := os.Getenv("WF_EE_LOG_FILE_PATH")
	if eeLogPath == "" {
		log.Fatal("environment variable 'WF_EE_LOG_FILE_PATH' is not set")
	}

	file, err := os.Open(eeLogPath)
	if err != nil {
		log.Fatalf("error occured while opening file: %v", err)
	}
	defer file.Close()

	// We want to replace 'host.docker.internal' with localhost as the user needs to navigate to this
	// outside of the container, accessing host.docker.internal on the host machine will not work.
	// We need to keep this value as 'host.docker.internal' for the rest of the program,
	// as this is used for http requests from inside the contianer.
	fmt.Printf("Please authenticate with discord via: %s/api/v1/discord/authorize\n", strings.ReplaceAll(apiBaseUrl, "host.docker.internal", "localhost"))

	// Wait for user to authenticate with discord
	for discordBearerToken == "" {
		time.Sleep(1 * time.Second)
		continue
	}

	fmt.Println("Successfully authenticated with Discord.")

	reader := bufio.NewReader(file)
	file.Seek(0, io.SeekEnd)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				time.Sleep(1 * time.Second)
				continue
			} else {
				fmt.Printf("error occured while reading line: %v", err)
				break
			}
		}

		if r.MatchString(line) {
			matches := r.FindStringSubmatch(line)
			username := removeNonPrintableCharacters(matches[2])

			fmt.Printf("Received DM from %s\n", username)

			if err := sendDiscordMessage(discordBearerToken, username); err != nil {
				fmt.Println(err.Error())
			}
		}
	}
}

func removeNonPrintableCharacters(val string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) {
			return r
		}
		return -1
	}, val)
}

func sendDiscordMessage(token string, username string) error {
	content := SendChannelMessageRequest{Content: fmt.Sprintf("You received a new direct message from __**%s**__", username)}

	body, err := json.Marshal(content)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/v1/discord/channels/@me/messages", apiBaseUrl)
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	client := &http.Client{}
	res, err := client.Do(request)
	if err != nil {
		return err
	}

	defer res.Body.Close()

	if res.StatusCode >= 400 {
		errorMessage := &ErrorResponse{}

		err := json.NewDecoder(res.Body).Decode(&errorMessage)
		if err != nil {
			return err
		}

		return fmt.Errorf("error occured while sending discord message: %s. Please ensure you have at least one mutual server with the Discord Bot.", errorMessage.Message)
	}

	return nil
}
