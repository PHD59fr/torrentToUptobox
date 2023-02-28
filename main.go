package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/net/proxy"
)

type ConfigFile struct {
	AlldebridAgent    string `json:"alldebrid_agent"`
	AlldebridAPIKey   string `json:"alldebrid_api_key"`
	UtbAPIKey         string `json:"utb_api_key"`
	TorrentDirectory  string `json:"torrent_directory"`
	FinishedDirectory string `json:"finished_directory"`
	ErrorDirectory    string `json:"error_directory"`
	ExpiredDirectory  string `json:"expired_directory"`
	ExpirationTime    string `json:"expiration_time"`
	OutputFile        string `json:"output_file"`
	AlldebridProxy    struct {
		Enabled  bool   `json:"enabled"`
		Type     string `json:"type"`
		Server   string `json:"server"`
		Port     int    `json:"port"`
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"alldebrid_proxy"`
}

type Magnet struct {
	Status string `json:"status"`
	Data   struct {
		Files []struct {
			File  string `json:"file"`
			Name  string `json:"name,omitempty"`
			Size  int    `json:"size,omitempty"`
			Hash  string `json:"hash,omitempty"`
			Ready bool   `json:"ready,omitempty"`
			ID    int    `json:"id,omitempty"`
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error,omitempty"`
		} `json:"files"`
	} `json:"data"`
}

type FinishedMagnet struct {
	Status string `json:"status"`
	Data   struct {
		Magnets struct {
			ID             int    `json:"id"`
			Filename       string `json:"filename"`
			Size           int64  `json:"size"`
			Hash           string `json:"hash"`
			Status         string `json:"status"`
			StatusCode     int    `json:"statusCode"`
			Downloaded     int64  `json:"downloaded"`
			Uploaded       int    `json:"uploaded"`
			Seeders        int    `json:"seeders"`
			DownloadSpeed  int    `json:"downloadSpeed"`
			ProcessingPerc int    `json:"processingPerc"`
			UploadSpeed    int    `json:"uploadSpeed"`
			UploadDate     int    `json:"uploadDate"`
			CompletionDate int    `json:"completionDate"`
			Links          []struct {
				Link     string `json:"link"`
				Filename string `json:"filename"`
				Size     int64  `json:"size"`
				Files    []struct {
					N string `json:"n"`
					S int64  `json:"s"`
				} `json:"files"`
			} `json:"links"`
			Type     string `json:"type"`
			Notified bool   `json:"notified"`
			Version  int    `json:"version"`
		} `json:"magnets"`
	} `json:"data"`
}

type MagnetsList struct {
	Status string `json:"status"`
	Data   struct {
		Magnets []struct {
			ID             int    `json:"id"`
			Filename       string `json:"filename"`
			Size           int    `json:"size"`
			Hash           string `json:"hash"`
			Status         string `json:"status"`
			StatusCode     int    `json:"statusCode"`
			Downloaded     int    `json:"downloaded"`
			Uploaded       int    `json:"uploaded"`
			Seeders        int    `json:"seeders"`
			DownloadSpeed  int    `json:"downloadSpeed"`
			ProcessingPerc int    `json:"processingPerc"`
			UploadSpeed    int    `json:"uploadSpeed"`
			UploadDate     int64  `json:"uploadDate"`
			CompletionDate int    `json:"completionDate"`
			Links          []struct {
				Link     string `json:"link"`
				Filename string `json:"filename"`
				Size     int    `json:"size"`
				Files    []struct {
					N string `json:"n"`
					S int    `json:"s"`
				} `json:"files"`
			} `json:"links"`
			Type     string `json:"type"`
			Notified bool   `json:"notified"`
			Version  int    `json:"version"`
		} `json:"magnets"`
	} `json:"data"`
}

type UTBLink struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
	Data       struct {
		DlLink string `json:"dlLink"`
	} `json:"data"`
}

var filename string = "torrentToUTB.log"
var f, _ = os.OpenFile(filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
var multilog = io.MultiWriter(os.Stdout, f)

var log = &logrus.Logger{
	Out:   multilog,
	Level: logrus.InfoLevel,
	Formatter: &logrus.TextFormatter{
		DisableColors:   false,
		TimestampFormat: "2006-01-02 15:04:05",
		FullTimestamp:   true,
	},
}

var config = readConfigFile("config.json")
var endpointAlldebridStatus = "https://api.alldebrid.com/v4/magnet/status?agent=" + config.AlldebridAgent + "&apikey=" + config.AlldebridAPIKey

func readConfigFile(filename string) ConfigFile {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal("File reading error: ", err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Fatal("File closing error: ", err)
		}
	}(file)

	var config ConfigFile
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		log.Fatal("File ", filename, " error: ", err)
	}
	return config
}

func initHttpClient(config ConfigFile) http.Client {
	tr := &http.Transport{
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,
	}
	myproxy := config.AlldebridProxy
	if myproxy.Enabled {
		switch myproxy.Type {
		case "socks5":
			var dialer proxy.Dialer
			if myproxy.Server != "" {
				var err error
				if myproxy.Username != "" && myproxy.Password != "" {
					auth := proxy.Auth{
						User:     myproxy.Username,
						Password: myproxy.Password,
					}
					myproxy.Server = myproxy.Server + ":" + fmt.Sprintf("%d", myproxy.Port)
					dialer, err = proxy.SOCKS5("tcp", myproxy.Server, &auth, proxy.Direct)
					if err != nil {
						log.Fatal("Error connecting to proxy:", err)
					}
				} else {
					dialer, err = proxy.SOCKS5("tcp", myproxy.Server+":"+fmt.Sprintf("%d", myproxy.Port), nil, proxy.Direct)
					if err != nil {
						log.Fatal("Error connecting to proxy:", err)
					}
				}
			}
			tr = &http.Transport{Dial: dialer.Dial}
		}
	}
	myClient := http.Client{Transport: tr}
	return myClient
}

func getAllDebridUrl(url string) ([]byte, int) {
	client := http.Client{}
	client = initHttpClient(config)
	resp, err := client.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(resp.Body)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	return body, resp.StatusCode
}

func deleteMagnet(id int) {
	endpointDelete := "https://api.alldebrid.com/v4/magnet/delete?agent=" + config.AlldebridAgent + "&apikey=" + config.AlldebridAPIKey + "&id=" + fmt.Sprintf("%d", id)
	result, code := getAllDebridUrl(endpointDelete)
	if code != 200 {
		log.Fatal(result)
	}
}

func getTorrentList() []string {
	log.Infoln("Scanning folder", config.TorrentDirectory+" ...")
	files, err := filepath.Glob(config.TorrentDirectory + "*.torrent")
	if err != nil {
		log.Fatal(err)
	}

	if len(files) == 0 {
		log.Infoln("FILESYSTEM - No file detected !")
	} else if len(files) == 1 {
		log.Infoln(len(files), "FILESYSTEM - file detected !")
	} else {
		log.Infoln(len(files), "FILESYSTEM - files detected !")
	}
	return files
}

func CleanInactiveMagnet() {
	expireTime, err := time.ParseDuration(config.ExpirationTime)
	if err != nil {
		log.Fatalln(err)
	}

	timestamp := time.Now().Unix()
	maxDownloadTimeOut := timestamp - int64(expireTime.Seconds())

	// ON FILESYSTEM
	log.Println()
	log.Infoln("##### CleanInactiveMagnet Step")
	fileList := getTorrentList()

	for _, file := range fileList {
		fileStat, err := os.Stat(file)
		if err != nil {
			log.Fatal(err)
		}
		modifiedtime := fileStat.ModTime().Unix()
		if modifiedtime <= maxDownloadTimeOut {
			destination := config.ExpiredDirectory + filepath.Base(file)
			log.Warningln("FILESYSTEM - REMOVE INACTIVE", file)
			err := os.Rename(file, destination)
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	// ON ALLDEBRID
	result, code := getAllDebridUrl(endpointAlldebridStatus)
	if code != 200 {
		log.Fatal(result)
	}
	var magnetsList MagnetsList
	err = json.Unmarshal(result, &magnetsList)
	if err != nil {
		log.Fatal(err)
	}

	// Status list https://docs.alldebrid.com/#status
	if magnetsList.Status == "success" {
		for _, magnet := range magnetsList.Data.Magnets {
			if magnet.Status != "Ready" {
				if magnet.StatusCode == 10 { // Alldebrid 72h inactive torrent
					log.Warningln("ALLDEBRID - REMOVE INACTIVE", magnet.Filename)
					deleteMagnet(magnet.ID)
					continue
				}
				if magnet.StatusCode > 4 { // 0 to 4 Download to Ready, 5 to 11 Error (except 10)
					log.Error(magnet)
				} else {
					if magnet.UploadDate <= maxDownloadTimeOut && magnet.Downloaded == 0 {
						log.Warningln("ALLDEBRID - REMOVE INACTIVE", magnet.Filename)
						deleteMagnet(magnet.ID)
					}
				}
			}
		}
	}
}

func prepareUpload(uri string, paramName, path string) (*http.Request, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(paramName, filepath.Base(path))
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(part, file)

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", uri, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, err
}

func UploadMagnet() {
	urlUpload := "https://api.alldebrid.com/v4/magnet/upload/file?agent=" + config.AlldebridAgent + "&apikey=" + config.AlldebridAPIKey
	urlUtb := "https://uptobox.com/api/link?token="
	log.Println()
	log.Infoln("##### UploadMagnet Step")
	fileList := getTorrentList()

	for _, file := range fileList {
		request, _ := prepareUpload(urlUpload, "files[0]", file)

		client := initHttpClient(config)
		resp, err := client.Do(request)
		if err != nil {
			log.Fatal(err)
		} else {
			body := &bytes.Buffer{}
			_, err := body.ReadFrom(resp.Body)
			if err != nil {
				log.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != 200 {
				log.Fatal(resp.StatusCode, "-", body)
			}

			var magnet Magnet
			decoder := json.NewDecoder(body)
			err = decoder.Decode(&magnet)
			if err != nil {
				log.Fatal(err)
			}

			if magnet.Status == "success" {
				// File is uploaded
				if magnet.Data.Files[0].Ready == false {
					continue
				}
				urlMagnet := endpointAlldebridStatus + "&id=" + fmt.Sprintf("%d", magnet.Data.Files[0].ID)
				result, code := getAllDebridUrl(urlMagnet)
				if code != 200 {
					log.Fatal(result)
				}
				var finishedMagnet FinishedMagnet
				err = json.Unmarshal(result, &finishedMagnet)
				if err != nil {
					log.Fatal(err)
				}

				if finishedMagnet.Status == "success" {
					if finishedMagnet.Data.Magnets.Status != "Ready" { // Check if all files are ok !
						log.Infoln(file, "found but not ready !")
						continue
					}
					magnets := finishedMagnet.Data.Magnets.Links
					for _, magnet := range magnets {
						utbId := filepath.Base(magnet.Link)
						utbUrlFile := urlUtb + config.UtbAPIKey + "&file_code=" + utbId
						utbBody, err := http.Get(utbUrlFile)
						if err != nil {
							log.Fatal(err)
						}

						body := &bytes.Buffer{}
						_, err = body.ReadFrom(utbBody.Body)
						if err != nil {
							log.Fatal(err)
						}
						utbBody.Body.Close()
						if utbBody.StatusCode != 200 {
							log.Fatal(utbBody.StatusCode, "-", body)
						}

						var utbLink UTBLink
						decoder := json.NewDecoder(body)
						err = decoder.Decode(&utbLink)
						if err != nil {
							log.Error(err)
						}
						if utbLink.StatusCode == 1 {
							// if error
							log.Error("Error on torrent ", file, ", file: ", magnet.Filename, " - ", utbLink.Message)
						} else {
							f, err := os.OpenFile(config.OutputFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
							if err != nil {
								log.Fatal(err)
							}
							if _, err := f.WriteString(utbLink.Data.DlLink + "\n"); err != nil {
								log.Fatal(err)
							}
							f.Close()
							log.Infoln("Write final link", utbLink.Data.DlLink, "for download")
						}
					}
					finalFile := config.FinishedDirectory + filepath.Base(file)
					os.Rename(file, finalFile)
					log.Infoln("file", file, "moved to", config.FinishedDirectory)
				} else {
					// finishedMagnet return not a success
				}
			}
		}
	}
}

func main() {
	// Create folder Error, Finished, Expired
	for _, folder := range []string{config.ErrorDirectory, config.FinishedDirectory, config.ExpiredDirectory} {
		if _, err := os.Stat(folder); errors.Is(err, os.ErrNotExist) {
			err := os.Mkdir(folder, os.ModePerm)
			if err != nil {
				log.Fatalln(err)
			}
		}
	}

	CleanInactiveMagnet() // Clean if inactive
	UploadMagnet()        // upload magnet file

}
