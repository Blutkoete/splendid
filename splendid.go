package main

import (
	"bufio"
	"encoding/json"
	"github.com/bpicode/fritzctl/fritz"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

var username string
var password string
var authorizedKeys[]string
var fritzbox fritz.HomeAuto

type homeAutomationRequest struct {
	Key string
	Device string
	Name string
	Action string
	Value string
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

func requestDispatcher(writer http.ResponseWriter, request *http.Request) {
	log.Printf("Received %s request from %s.\n", request.Method, request.RemoteAddr)

	if request.Method != "POST" {
		log.Println(request.Body)
		log.Println("405 - Method not allowed")
		writer.WriteHeader(http.StatusMethodNotAllowed)
		writer.Write([]byte("405 - Method not allowed"))
		return
	}

	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		log.Println(request.Body)
		log.Println(err)
		log.Println("400 - Invalid body")
		writer.WriteHeader(http.StatusBadRequest)
		writer.Write([]byte("400 - Invalid body"))
		return
	}

	var parsedRequest homeAutomationRequest
	err = json.Unmarshal(body, &parsedRequest)
	if err != nil {
		log.Println(string(body))
		log.Println(err)
		log.Println("400 - Invalid body")
		writer.WriteHeader(http.StatusBadRequest)
		writer.Write([]byte("400 - Invalid body"))
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
		log.Println(parsedRequest.Key)
		log.Println("403 - Permission denied")
		writer.WriteHeader(http.StatusForbidden)
		writer.Write([]byte("403 - Permission denied"))
		return
	}

	fritzbox = fritz.NewHomeAuto(
		fritz.SkipTLSVerify(),
		fritz.Credentials(username, password),
	)
	err = fritzbox.Login()
	if err != nil {
		log.Fatal(err)
	}

	if parsedRequest.Device == "switch" {
		if parsedRequest.Action == "set" {
			if parsedRequest.Value == "0" {
				err = fritzbox.Off(parsedRequest.Name)
			} else if parsedRequest.Value == "1" {
				err = fritzbox.On(parsedRequest.Name)
			} else {
				log.Printf("{ \"key\": <VALID>, \"device\": \"%s\", \"name\": \"%s\", \"action\": \"%s\", \"value\": \"%s\" }",
					       parsedRequest.Device, parsedRequest.Name, parsedRequest.Action, parsedRequest.Value)
				log.Println("400 - Invalid body")
				writer.WriteHeader(http.StatusBadRequest)
				writer.Write([]byte("400 - Invalid body"))
				return
			}
		}
	}

	if err != nil {
		log.Printf("{ \"key\": <VALID>, \"device\": \"%s\", \"name\": \"%s\", \"action\": \"%s\", \"value\": \"%s\" }",
			parsedRequest.Device, parsedRequest.Name, parsedRequest.Action, parsedRequest.Value)
		log.Println(err)
		log.Println("406 - Request not acceptable")
		writer.WriteHeader(http.StatusNotAcceptable)
		writer.Write([]byte("406 - Request not acceptable"))
		return
	}

	log.Printf("{ \"key\": <VALID>, \"device\": \"%s\", \"name\": \"%s\", \"action\": \"%s\", \"value\": \"%s\" }",
		parsedRequest.Device, parsedRequest.Name, parsedRequest.Action, parsedRequest.Value)
	log.Println("200 - Ok")
	writer.WriteHeader(http.StatusOK)
	writer.Write([]byte("200 - Ok"))
}

func main() {
	var err error
	var credentials []string

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
	err = http.ListenAndServeTLS("10.13.5.20:7778", "/etc/splendid/cert.pem", "/etc/splendid/key.pem", nil)
	if err != nil {
		log.Fatal(err)
	}
}

