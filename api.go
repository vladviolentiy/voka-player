package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type Apicall struct{}

var endpoint = "https://api.voka.tv/"
var accessToken = ""

func executeGet(url string) []byte {
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Println("error to prepare get query", err)
		return nil
	}
	request.Header.Set("X-Forwarded-For", "37.215.1.1")
	request.Header.Set("User-Agent", "Voka-player")
	client := http.Client{}
	response, err := client.Do(request)
	if err != nil {
		log.Println("error to execute get query. ", err)
		return nil
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			return
		}
	}(response.Body)

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil
	}
	return body
}

func executePost(url string, params interface{}) []byte {
	decodedData, err := json.Marshal(params)
	if err != nil {
		fmt.Println("decodedData convert error:", err)
		return nil
	}
	request, err := http.NewRequest("POST", endpoint+url, bytes.NewBuffer(decodedData))
	if err != nil {
		log.Println("error to prepare post query", err)
		return nil
	}
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	request.Header.Set("X-Forwarded-For", "37.215.1.1")
	request.Header.Set("User-Agent", "Voka-player")
	client := http.Client{}
	response, err := client.Do(request)
	if err != nil {
		log.Println("error to execute post query. ", err)
		return nil
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			return
		}
	}(response.Body)

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil
	}
	return body
}

func executeEndpoint(call string) []byte {
	return executeGet(endpoint + "v1/" + call)
}

func (a Apicall) auth(params struct {
	ClientId     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	GrantType    string `json:"grant_type"`
	Password     string `json:"password"`
	Username     string `json:"username"`
}) []byte {
	return executePost("oauth/token?client_id=3e28685c-fce0-4994-9d3a-1dad2776e16a&client_version=4.2.0.160&locale=ru-RU&timezone=0", params)
}

func (a Apicall) setAccessToken(token string) {
	accessToken = token
}

func (a Apicall) downloadPlaylist() []byte {
	return executeEndpoint("collection_items.json?client_version=0.0.1&expand[channel]=genres,genres.images,images,live_preview,language,live_stream,catchup_availability,timeshift_availability,certification_ratings&filter[collection_id_eq]=9fc67851-41a1-429d-b7ca-4b8f49c53659&locale=ru-RU&page[limit]=300&page[offset]=0&sort=relevance&timezone=10800&client_id=66797942-ff54-46cb-a109-3bae7c855370")
}

func (a Apicall) getStream(channelId string) []byte {
	return executeEndpoint("channels/" + channelId + "/stream.json?drm=spbtvcas&screen_width=1920&video_codec=h264&audio_codec=mp4a&screen_height=1080&protocol=hls&device_token=eab6d977-a45f-4f83-820a-6539b6b7b463&locale=ru-RU&client_version=3.1.4-5984&timezone=10800&client_id=66797942-ff54-46cb-a109-3bae7c855370&access_token=" + accessToken)
}
