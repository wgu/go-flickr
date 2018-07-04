package main

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"time"

	"github.com/wgu/go-flickr/flickr"
)

type photo struct {
	Id    string `xml:"id,attr"`
	Title string `xml:"title,attr"`
}

type photoset struct {
	Id    string  `xml:"id,attr"`
	Title string  `xml:"title"`
	Photo []photo `xml:"photo"`
}

type photosets struct {
	Photoset []photoset `xml:"photoset"`
}

type collections struct {
	Collection []collection `xml:"collection"`
}

type collection struct {
	Id    string `xml:"id,attr"`
	Title string `xml:"title,attr"`
}

func main() {
	requestTemplate, err := flickr.NewRequestFromCmd()
	checkErr(err)

	var photosetid string
	uploadedPhotoSet := photoset{}

	// Specified album name to upload photos to or be created
	if requestTemplate.Album != "" {
		args := map[string]string{
			"method": "flickr.photosets.getList",
		}
		request := flickr.NewRequest(http.MethodGet, requestTemplate.Auth, args, requestTemplate.Secret)
		response, err := request.ExecuteWithRetry(2, time.Second)
		checkErr(err, response)
		var photoSets photosets
		checkErr(xml.Unmarshal([]byte(response), &photoSets), response)
		for _, photoSet := range photoSets.Photoset {
			if photoSet.Title != requestTemplate.Album {
				continue
			}
			photosetid = photoSet.Id
			args = map[string]string{
				"method":      "flickr.photosets.getPhotos",
				"photoset_id": photosetid,
			}
			request = flickr.NewRequest(http.MethodGet, requestTemplate.Auth, args, requestTemplate.Secret)
			response, err = request.ExecuteWithRetry(2, time.Second)
			checkErr(err, response)
			checkErr(xml.Unmarshal([]byte(response), &uploadedPhotoSet), response)
			break
		}
	}

	files, err := ioutil.ReadDir(requestTemplate.Dir)
	checkErr(err)
	for _, fileinfo := range files {
		filename := fileinfo.Name()
		filenameExt := filepath.Ext(filename)
		filenameBase := filename[:len(filename)-len(filenameExt)]

		// Album already exists
		if photosetid != "" {
			var uploaded bool
			for _, p := range uploadedPhotoSet.Photo {
				if filenameBase == p.Title {
					uploaded = true
				}
			}
			if uploaded {
				fmt.Println("Already exists: " + filename)
				continue
			}
		}

		fmt.Println("Uploading " + filename)
		photopath := filepath.Join(requestTemplate.Dir, filename)
		request := flickr.NewRequest(http.MethodPost, requestTemplate.Auth, nil, requestTemplate.Secret)
		photoid, err := request.UploadWithRetry(photopath, 2, time.Second)
		if err != nil && err.Error() == photopath+" is not an image." {
			fmt.Println(err.Error() + " Skipped...")
			continue
		}
		checkErr(err)

		// No album yet
		if photosetid == "" {
			fmt.Println("Creating album")
			title := requestTemplate.Album
			if title == "" {
				title = filepath.Base(requestTemplate.Dir)
			}
			additionalArgs := map[string]string{
				"method":           "flickr.photosets.create",
				"title":            title,
				"primary_photo_id": photoid,
			}
			request = flickr.NewRequest(http.MethodPost, requestTemplate.Auth, additionalArgs, requestTemplate.Secret)
			response, err := request.ExecuteWithRetry(2, time.Second)
			checkErr(err, response)
			// fmt.Println(response)
			var pset photoset
			checkErr(xml.Unmarshal([]byte(response), &pset), response)
			photosetid = pset.Id
			fmt.Println("Photaset id: " + photosetid)
		} else {
			fmt.Println("Adding " + photoid + " to album")
			additionalArgs := map[string]string{
				"method":      "flickr.photosets.addPhoto",
				"photoset_id": photosetid,
				"photo_id":    photoid,
			}
			request = flickr.NewRequest(http.MethodPost, requestTemplate.Auth, additionalArgs, requestTemplate.Secret)
			response, err := request.ExecuteWithRetry(2, time.Second)
			checkErr(err, response)
		}
	}

	if photosetid != "" && requestTemplate.Collection != "" {
		additionalArgs := map[string]string{
			"method": "flickr.collections.getTree",
		}
		request := flickr.NewRequest(http.MethodGet, requestTemplate.Auth, additionalArgs, requestTemplate.Secret)
		response, err := request.ExecuteWithRetry(2, time.Second)
		checkErr(err, response)
		var cs collections
		checkErr(xml.Unmarshal([]byte(response), &cs), response)
		var collectionId string
		for _, c := range cs.Collection {
			if c.Title == requestTemplate.Collection {
				collectionId = c.Id
				break
			}
		}
		if collectionId == "" {
			fmt.Println("Creating collection " + requestTemplate.Collection)
			additionalArgs := map[string]string{
				"method": "flickr.collections.create",
				"title":  requestTemplate.Collection,
			}
			request := flickr.NewRequest(http.MethodPost, requestTemplate.Auth, additionalArgs, requestTemplate.Secret)
			response, err := request.ExecuteWithRetry(2, time.Second)
			checkErr(err, response)
			var c collection
			checkErr(xml.Unmarshal([]byte(response), &c))
			collectionId = c.Id
		}
		fmt.Println("Adding album " + photosetid + " to collection")
		additionalArgs = map[string]string{
			"method":        "flickr.collections.addSet",
			"collection_id": collectionId,
			"photoset_id":   photosetid,
		}
		request = flickr.NewRequest(http.MethodPost, requestTemplate.Auth, additionalArgs, requestTemplate.Secret)
		response, err = request.ExecuteWithRetry(2, time.Second)
		if err != nil && err.Error() == "4: Set already in collection" {
			fmt.Println("Album already in collection")
		} else {
			checkErr(err, response)
		}
	}
}

func checkErr(e error, msg ...string) {
	if e != nil {
		for m := range msg {
			fmt.Println(m)
		}
		panic(e)
	}
}
