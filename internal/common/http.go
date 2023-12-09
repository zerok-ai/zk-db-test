package common

import (
	"fmt"
	zkLogger "github.com/zerok-ai/zk-utils-go/logs"
	"io"
	"io/ioutil"
	"net/http"
)

const LogTag = "common"

func MakeHTTPCall(url string) (string, error) {
	// Make the GET request
	response, err := http.Get(url)
	if err != nil {
		fmt.Printf("Error making GET request: %v\n", err)
		return "", err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			zkLogger.ErrorF(LogTag, "Error closing response body: %v\n", err)
		}
	}(response.Body)

	// Read the response body
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		fmt.Printf("Error reading response body: %v\n", err)
		return "", err
	}
	return string(body), nil
}
