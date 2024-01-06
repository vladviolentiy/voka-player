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

type ConfigFile struct {
	Login     string `json:"login"`
	Password  string `json:"password"`
	IpAddress string `json:"ipAddress"`
	Port      int    `json:"port"`
}

type AccessTokenCache struct {
	AccessToken string `json:"access_token"`
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

var configuration ConfigFile

func main() {
	readConfigFile()
	checkIssetCurrentToken()

	rtr := mux.NewRouter()
	rtr.HandleFunc("/play/{channelId:[0-9a-f-]+}/{quality:[0-9]}/playlist.m3u8", playPlaylistFixed).Methods("GET")
	rtr.HandleFunc("/play/{channelId:[0-9a-f-]+}/max/playlist.m3u8", playPlaylistMax).Methods("GET")
	rtr.HandleFunc("/play/{channelId:[0-9a-f-]+}/{quality:[0-9]}/{file:[0-9a-z-]+}.ts", getTs).Methods("GET")
	rtr.HandleFunc("/play/{channelId:[0-9a-f-]+}/max/{file:[0-9a-z-]+}.ts", getTs).Methods("GET")
	rtr.HandleFunc("/playlist.m3u8", getPlaylist).Methods("GET")
	rtr.HandleFunc("/download.m3u8", downloadPlaylist).Methods("GET")
	http.Handle("/", rtr)
	log.Println("Listening...")
	err := http.ListenAndServe(":"+strconv.Itoa(configuration.Port), nil)
	if err != nil {
		fmt.Println("error: ", err)
		return
	}
}

func checkIssetCurrentToken() {
	if _, err := os.Stat("cache.json"); err == nil {
		fmt.Println("File isset. Load Token From File")

		file, err := os.Open("cache.json")
		if err != nil {
			fmt.Println("error: ", err)
			return
		}
		defer file.Close()

		decoder := json.NewDecoder(file)
		var accessTokenCache AccessTokenCache
		err = decoder.Decode(&accessTokenCache)
		if err != nil {
			fmt.Println("error: ", err)
		}

		if configuration.Login == "" {
			fmt.Println("Login is empty")
			return
		}
		api.setAccessToken(accessTokenCache.AccessToken)
	} else if os.IsNotExist(err) {
		fmt.Println("Cache file not exists. Creating")

		jsonString, _ := json.Marshal(AccessTokenCache{
			AccessToken: getFreshToken(),
		})
		err := os.WriteFile("cache.json", jsonString, 0644)
		if err != nil {
			return
		}
	} else {
		fmt.Println("error: ", err)
	}

}

func readConfigFile() {
	// read config file
	file, err := os.Open("config.json")
	if err != nil {
		fmt.Println("error: ", err)
		return
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&configuration)
	if err != nil {
		fmt.Println("error: ", err)
		os.Exit(1)
	}

	if configuration.Login == "" {
		fmt.Println("Login is empty")
		os.Exit(1)
	}

	if configuration.Password == "" {
		fmt.Println("Password is empty")
		os.Exit(1)
	}

}

func getFreshToken() string {
	response := api.auth(struct {
		ClientId     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		GrantType    string `json:"grant_type"`
		Password     string `json:"password"`
		Username     string `json:"username"`
	}{ClientId: "3e28685c-fce0-4994-9d3a-1dad2776e16a", ClientSecret: "0eef32c1-7c25-45ba-b31e-215bd8555d7a", GrantType: "password", Password: configuration.Password, Username: configuration.Login})

	var responseDecoded AccessTokenCache

	marshalError := json.Unmarshal(response, &responseDecoded)
	if marshalError != nil {
		log.Println("error decoding auth", marshalError)
		return ""
	}
	return responseDecoded.AccessToken
}

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
		playlistBuilder.WriteString("http://" + configuration.IpAddress + ":" + strconv.Itoa(configuration.Port) + "/play/" + data.Id + "/max/playlist.m3u8" + "\n")
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
