package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type streamJsonInterface struct {
	Data struct {
		Url string `json:"url"`
	} `json:"data"`
}

type downloadPlaylistInterface struct {
	Data []struct {
		Id   string `json:"id"`
		Name string `json:"name"`
	}
}

var api = Apicall{}

var hashList = make(map[string]string)
var timeOutList = make(map[string]int32)

func main() {
	rtr := mux.NewRouter()
	rtr.HandleFunc("/play/{channelId:[0-9a-f-]+}/{quality:[0-9]}/playlist.m3u8", playPlaylistFixed).Methods("GET")
	rtr.HandleFunc("/play/{channelId:[0-9a-f-]+}/max/playlist.m3u8", playPlaylistMax).Methods("GET")
	rtr.HandleFunc("/play/{channelId:[0-9a-f-]+}/{quality:[0-9]}/{file:[0-9a-z-]+}.ts", getTs).Methods("GET")
	rtr.HandleFunc("/play/{channelId:[0-9a-f-]+}/max/{file:[0-9a-z-]+}.ts", getTs).Methods("GET")
	rtr.HandleFunc("/playlist.m3u8", getPlaylist).Methods("GET")
	rtr.HandleFunc("/download.m3u8", downloadPlaylist).Methods("GET")

	http.Handle("/", rtr)

	log.Println("Listening...")
	err := http.ListenAndServe(":3000", nil)
	if err != nil {
		return
	}
}

//
//func auth() {
//	uuidRandom, randomError := uuid.NewRandom()
//	if randomError != nil {
//		return
//	}
//	var randomString = uuidRandom.String()
//	data := []byte(`{"foo":"bar"}`)
//	r := bytes.NewReader(data)
//	response, err := http.Post("https://api.voka.tv/oauth/token?client_id="+randomString, "application/json", r)
//	if err != nil {
//		return
//	}
//	defer response.Body.Close()
//}

func playPlaylistFixed(w http.ResponseWriter, r *http.Request) {
	playPlaylist(w, r, false)
}
func playPlaylistMax(w http.ResponseWriter, r *http.Request) {
	playPlaylist(w, r, true)
}

func playPlaylist(w http.ResponseWriter, r *http.Request, maxQuality bool) {
	w.Header().Add("Content-Type", "application/vnd.apple.mpegurl")
	params := mux.Vars(r)
	channelId := params["channelId"]
	quality := params["quality"]
	if timeOutList[channelId] < int32(time.Now().Unix()) {
		delete(hashList, channelId)
	}
	var linkUrl string
	if val, ok := hashList[channelId]; ok {
		log.Println("Hash code isset in cache, get info from cache")
		linkUrl = val
	} else {
		log.Println("Hash code not isset in cache, Downloading, parsing, put in cache")
		body := api.getStream(channelId)
		var streamJsonResponse streamJsonInterface
		var err = json.Unmarshal(body, &streamJsonResponse)
		if err != nil {
			log.Print(err)
			_, err := w.Write([]byte("Error JSON parsing"))
			if err != nil {
				return
			}
			return
		}
		linkUrl = streamJsonResponse.Data.Url
		hashList[channelId] = linkUrl
		timeOutList[channelId] = int32(time.Now().Unix()) + 3600*3
	}
	var linkSplited = strings.Split(linkUrl, "/")
	var linkBuilder strings.Builder
	var channelBasicId string

	for index, element := range linkSplited {
		if index < len(linkSplited)-1 {
			linkBuilder.WriteString(element + "/")
		}
		if index == 3 {
			decodedString, err := url.QueryUnescape(element)
			channelParams, err := base64.StdEncoding.DecodeString(decodedString)
			if err != nil {
				fmt.Println("Ошибка разбора base64")
				fmt.Println(err)
				fmt.Println(element)
				return
			}
			var jsonResult2 map[string]interface{}
			jsonObjectError := json.Unmarshal(channelParams, &jsonResult2)
			if jsonObjectError != nil {
				fmt.Println("Ошибка разбора json")
				return
			}
			channelBasicId = jsonResult2["stream_name"].(string)
		}
	}

	if maxQuality {
		body := executeGet(linkUrl)
		re := regexp.MustCompile(`iframes-vid-(\d+)k_v5\.m3u8`)
		matchCount := len(re.FindAll(body, -1))
		quality = strconv.Itoa(matchCount)
	}

	linkBuilder.WriteString(channelBasicId + "-inadv-qidx-" + quality + "k_v3.m3u8")
	body := executeGet(linkBuilder.String())

	_, writeError := w.Write(body)
	if writeError != nil {
		return
	}
}

func getTs(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "video/mp2t")
	params := mux.Vars(r)
	channelId := params["channelId"]
	file := params["file"]

	linkUrl := hashList[channelId]

	var linkSplited = strings.Split(linkUrl, "/")
	var linkBuilder strings.Builder

	for index, element := range linkSplited {
		if index < len(linkSplited)-1 {
			linkBuilder.WriteString(element + "/")
		}
	}
	linkBuilder.WriteString(file + ".ts")
	body := executeGet(linkBuilder.String())
	_, writeError := w.Write(body)
	if writeError != nil {
		log.Println("Error write to ResponseWriter")
		return
	}
}

func downloadPlaylist(w http.ResponseWriter, r *http.Request) {
	body := api.downloadPlaylist()

	var downloadPlaylistResponse downloadPlaylistInterface

	marshalError := json.Unmarshal(body, &downloadPlaylistResponse)
	if marshalError != nil {
		return
	}
	var playlistBuilder strings.Builder
	playlistBuilder.WriteString("#EXTM3U\n")
	for _, data := range downloadPlaylistResponse.Data {
		playlistBuilder.WriteString("#EXTINF:-1," + data.Name + "\n")
		playlistBuilder.WriteString("http://127.0.0.1:3000/play/" + data.Id + "/max/playlist.m3u8" + "\n")
	}
	_, writeError := w.Write([]byte(playlistBuilder.String()))
	if writeError != nil {
		return
	}

}

func getPlaylist(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile("playlist.m3u8")
	if err != nil {
		fmt.Println(err)
		return
	}
	_, writeError := w.Write(data)
	if writeError != nil {
		return
	}
}
