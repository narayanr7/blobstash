package client

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"
)

// ErrBlobNotFound is returned from a get/stat request
// if the blob does not exist.
var ErrBlobNotFound = errors.New("blob not found")
var ErrKeyNotFound = errors.New("key doest not exist")

var defaultServerAddr = "http://localhost:8050"

// KeyValue holds a singke key value pair, along with the version (the creation timestamp)
type KeyValue struct {
	Key     string `json:"key,omitempty"`
	Value   string `json:"value"`
	Version int    `json:"version"`
}

// KeyValueVersions holds the full history for a key value pair
type KeyValueVersions struct {
	Key      string      `json:"key"`
	Versions []*KeyValue `json:"versions"`
}

type KvStore struct {
	ServerAddr string
	client     *http.Client
}

func NewKvStore(serverAddr string) *KvStore {
	if serverAddr == "" {
		serverAddr = defaultServerAddr
	}
	return &KvStore{
		ServerAddr: serverAddr,
		client:     &http.Client{},
	}
}

func (kvs *KvStore) Put(key, value string, version int) (*KeyValue, error) {

}

func (kvs *KvStore) Get(key string, version int) (*KeyValue, error) {

}

func (kvs *KvStore) Versions(key string, start, end, limit int) (*KeyValueVersions, error) {

}

func (kvs *KvStore) Keys(start, end string, limit int) ([]string, error) {

}

type BlobStore struct {
	ServerAddr string
	client     *http.Client
}

func NewBlobStore(serverAddr string) *BlobStore {
	if serverAddr == "" {
		serverAddr = defaultServerAddr
	}
	return &BlobStore{
		ServerAddr: serverAddr,
		client:     &http.Client{},
	}
}

// Get fetch the given blob.
func (bs *BlobStore) Get(hash string) ([]byte, error) {
	request, err := http.NewRequest("GET", bs.ServerAddr+"/api/v1/blobstore/blob/"+hash, nil)
	if err != nil {
		return nil, err
	}
	resp, err := bs.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	switch {
	case resp.StatusCode == 200:
		return body, nil
	case resp.StatusCode == 404:
		return nil, ErrBlobNotFound
	default:
		return nil, fmt.Errorf("failed to get blob %v: %v", hash, string(body))
	}
}

// Stat checks wether a blob exists or not.
func (bs *BlobStore) Stat(hash string) (bool, error) {
	request, err := http.NewRequest("HEAD", bs.ServerAddr+"/api/v1/blobstore/blob/"+hash, nil)
	if err != nil {
		return false, err
	}
	resp, err := bs.client.Do(request)
	if err != nil {
		return false, err
	}
	resp.Body.Close()
	switch {
	case resp.StatusCode == 200:
		return true, nil
	case resp.StatusCode == 404:
		return false, nil
	default:
		return false, fmt.Errorf("failed to put blob %v", hash)
	}
}

// Put upload the given blob, the caller is responsible for computing the SHA-1 hash
func (bs *BlobStore) Put(hash string, blob []byte) error {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(hash, hash)
	if err != nil {
		return err
	}
	if _, err := part.Write(blob); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	request, err := http.NewRequest("POST", bs.ServerAddr+"/api/v1/blobstore/upload", body)
	if err != nil {
		return err
	}
	resp, err := bs.client.Do(request)
	if err != nil {
		return err
	}
	body.Reset()
	body.ReadFrom(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to put blob %v", body.String())
	}
	return nil
}
