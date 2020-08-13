package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/bpicode/fritzctl/fritz"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

var username string
var password string
var authorizedKeys []string
var fritzbox fritz.HomeAuto

type homeAutomationRequest struct {
	Key    string
	Device string
	Name   string
	Action string
	Value  string
}

func readAllLines(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return lines, nil
}

func logTextAndError(text string, err error) {
	if text != "" {
		log.Println(text)
	}
	if err != nil {
		log.Println(err)
	}
}

func respondWithStatusCode(writer http.ResponseWriter, statusCode int, additionalText string) {
	writer.WriteHeader(statusCode)
	if additionalText != "" {
		log.Println(additionalText)
		writer.Write([]byte(additionalText))
	}
}

func requestDispatcher(writer http.ResponseWriter, request *http.Request) {
	log.Printf("Received %s request from %s.\n", request.Method, request.RemoteAddr)

	if request.Method != "POST" {
		logTextAndError("", nil)
		respondWithStatusCode(writer, http.StatusMethodNotAllowed, "405 - Method not allowed")
		return
	}

	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		logTextAndError("", err)
		respondWithStatusCode(writer, http.StatusBadRequest, "400 - Bad request")
		return
	}

	var parsedRequest homeAutomationRequest
	err = json.Unmarshal(body, &parsedRequest)
	if err != nil {
		logTextAndError(string(body), err)
		respondWithStatusCode(writer, http.StatusBadRequest, "400 - Bad request")
		return
	}

	keyValid := false
	for _, key := range authorizedKeys {
		if key == parsedRequest.Key {
			keyValid = true
			break
		}
	}

	if !keyValid {
		logTextAndError(string(body), nil)
		respondWithStatusCode(writer, http.StatusForbidden, "403 - Forbidden")
		return
	}

	logTextAndError(fmt.Sprintf("{ \"key\": <VALID>, \"device\": \"%s\", \"name\": \"%s\",  \"action\": \"%s\", \"value\": \"%s\" }",
		parsedRequest.Device, parsedRequest.Name, parsedRequest.Action, parsedRequest.Value), nil)

	fritzbox = fritz.NewHomeAuto(
		fritz.SkipTLSVerify(),
		fritz.Credentials(username, password),
	)
	err = fritzbox.Login()
	if err != nil {
		logTextAndError("", nil)
		respondWithStatusCode(writer, http.StatusInternalServerError, "500 - Internal server error")
		return
	}

	if parsedRequest.Device == "switch" {
		if parsedRequest.Action == "set" {
			if parsedRequest.Value == "0" {
				err = fritzbox.Off(parsedRequest.Name)
			} else if parsedRequest.Value == "1" {
				err = fritzbox.On(parsedRequest.Name)
			} else {
				respondWithStatusCode(writer, http.StatusNotAcceptable, "406 - Not acceptable")
				return
			}
		}
	} else {
		respondWithStatusCode(writer, http.StatusNotAcceptable, "406 - Not acceptable")
		return
	}

	if err != nil {
		logTextAndError("", err)
		respondWithStatusCode(writer, http.StatusNotAcceptable, "406 - Not acceptable")
		return
	}

	respondWithStatusCode(writer, http.StatusOK, "200 - Ok")
}

func main() {
	var err error
	var config []string
	var listenAddressAndPort string
	var certPath string
	var keyPath string
	var credentials []string

	logFile, err := os.OpenFile("/var/log/splendid/splendid.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Println(err)
	} else {
		defer logFile.Close()
		log.SetOutput(logFile)
	}

	log.Println("Starting up ...")

	config, err = readAllLines("/etc/splendid/config")
	if err != nil {
		log.Fatal(err)
	}

	if len(config) == 1 {
		listenAddressAndPort = config[0]
		certPath = "/etc/splendid/cert.pem"
		keyPath = "/etc/splendid/key.pem"
	} else if len(config) == 3 {
		listenAddressAndPort = config[0]
		certPath = config[1]
		keyPath = config[2]
	} else {
		log.Fatalf("Invalid line count in config file: %d", len(credentials))
	}

	credentials, err = readAllLines("/etc/splendid/credentials")
	if err != nil {
		log.Fatal(err)
	}

	if len(credentials) == 1 {
		username = ""
		password = credentials[0]
	} else if len(credentials) == 2 {
		username = credentials[0]
		password = credentials[1]
	} else {
		log.Fatalf("Invalid line count in credentials file: %d", len(credentials))
	}

	authorizedKeys, err = readAllLines("/etc/splendid/authorized_keys")
	if err != nil {
		log.Fatal(err)
	}

	if len(authorizedKeys) == 0 {
		log.Fatal("No authorized keys available.")
	}

	fritzbox = fritz.NewHomeAuto(
		fritz.SkipTLSVerify(),
		fritz.Credentials(username, password),
	)
	err = fritzbox.Login()
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/gghr/", requestDispatcher)
	err = http.ListenAndServeTLS(listenAddressAndPort, certPath, keyPath, nil)
	if err != nil {
		log.Fatal(err)
	}
}
