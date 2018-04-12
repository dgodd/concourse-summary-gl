package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"
)

func GetJSON(url string, data interface{}) error {
	client := http.Client{
		Timeout: time.Second * 2, // Maximum of 2 secs
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "concourse-summary-gl")

	res, getErr := client.Do(req)
	if getErr != nil {
		return err
	}

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		return err
	}

	return json.Unmarshal(body, data)
}
