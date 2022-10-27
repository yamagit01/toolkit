package toolkit

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func TestTools_RandomString(t *testing.T) {
	var testTools Tools

	s := testTools.RandomString(10)
	if len(s) != 10 {
		t.Error("wrong length random string returned")
	}
}

var uploadTests = []struct {
	name          string
	allowedTypes  []string
	renameFile    bool
	errorExpected bool
}{
	{
		name:          "allowed no rename",
		allowedTypes:  []string{"image/jpeg", "image/png"},
		renameFile:    false,
		errorExpected: false,
	},
	{
		name:          "allowed rename",
		allowedTypes:  []string{"image/jpeg", "image/png"},
		renameFile:    true,
		errorExpected: false,
	},
	{
		name:          "not allowed",
		allowedTypes:  []string{"image/jpeg"},
		renameFile:    false,
		errorExpected: true,
	},
}

func TestTools_UploadFiles(t *testing.T) {
	var imageFile = filepath.Join("testdata", "img.png")
	var uploadFolder = filepath.Join("testdata", "uploads")

	for _, e := range uploadTests {
		// set up a pipe to avoid buffering
		pr, pw := io.Pipe()
		writer := multipart.NewWriter(pw)

		wg := sync.WaitGroup{}
		wg.Add(1)

		go func() {
			defer wg.Done()
			defer writer.Close()

			// create the form data field 'file'
			part, err := writer.CreateFormFile("file", imageFile)
			if err != nil {
				t.Error(err)
			}

			f, err := os.Open(imageFile)
			if err != nil {
				t.Error(err)
			}
			defer f.Close()

			img, _, err := image.Decode(f)
			if err != nil {
				t.Error("error decoding image", err)
			}

			err = png.Encode(part, img)
			if err != nil {
				t.Error(err)
			}
		}()

		// read from the pipe which receives data
		req := httptest.NewRequest("POST", "/", pr)
		req.Header.Add("Content-Type", writer.FormDataContentType())

		var testTools Tools
		testTools.AllowedFileType = e.allowedTypes

		uploadedFiles, err := testTools.UploadFiles(req, uploadFolder, e.renameFile)
		if err != nil && !e.errorExpected {
			t.Errorf("%s: no error expected but received", e.name)
		}

		if err == nil && e.errorExpected {
			t.Errorf("%s: error expected but none received", e.name)
		}

		if !e.errorExpected {
			if _, err := os.Stat(filepath.Join(uploadFolder, uploadedFiles[0].NewFileName)); os.IsNotExist(err) {
				t.Errorf("%s: expected file to exist: %s", e.name, err.Error())
			}

			// clean up
			_ = os.Remove(filepath.Join(uploadFolder, uploadedFiles[0].NewFileName))
		}

		wg.Wait()
	}
}

func TestTools_UploadOneFile(t *testing.T) {
	var imageFile = filepath.Join("testdata", "img.png")
	var uploadFolder = filepath.Join("testdata", "uploads")

	// set up a pipe to avoid buffering
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		defer writer.Close()

		// create the form data field 'file'
		part, err := writer.CreateFormFile("file", imageFile)
		if err != nil {
			t.Error(err)
		}

		f, err := os.Open(imageFile)
		if err != nil {
			t.Error(err)
		}
		defer f.Close()

		img, _, err := image.Decode(f)
		if err != nil {
			t.Error("error decoding image", err)
		}

		err = png.Encode(part, img)
		if err != nil {
			t.Error(err)
		}
	}()

	// read from the pipe which receives data
	req := httptest.NewRequest("POST", "/", pr)
	req.Header.Add("Content-Type", writer.FormDataContentType())

	var testTools Tools

	uploadedFiles, err := testTools.UploadOneFile(req, uploadFolder, true)
	if err != nil {
		t.Errorf("no error expected but received")
	}

	if _, err := os.Stat(filepath.Join(uploadFolder, uploadedFiles.NewFileName)); os.IsNotExist(err) {
		t.Errorf("expected file to exist: %s", err.Error())
	}

	// clean up
	_ = os.Remove(filepath.Join(uploadFolder, uploadedFiles.NewFileName))

}

func TestTools_CreateDirIfNotExist(t *testing.T) {
	var testTools Tools

	err := testTools.CreateDirIfNotExist("./testdata/myDir")
	if err != nil {
		t.Error(err)
	}

	err = testTools.CreateDirIfNotExist("./testdata/myDir")
	if err != nil {
		t.Error(err)
	}

	_ = os.Remove("./testdata/myDir")
}

var slugTests = []struct {
	name          string
	s             string
	expected      string
	errorExpected bool
}{
	{name: "valid string", s: " now is the time ", expected: "now-is-the-time", errorExpected: false},
	{name: "empty string", s: "", expected: "", errorExpected: true},
	{name: "complex string", s: "Now is the TIME! + fish & such &^123", expected: "now-is-the-time-fish-such-123", errorExpected: false},
	{name: "japanese string", s: "こんにちは", expected: "", errorExpected: true},
	{name: "japanese string and roman characters", s: "hello world こんにちは", expected: "hello-world", errorExpected: false},
}

func TestTools_Slugify(t *testing.T) {
	var testTools Tools

	for _, e := range slugTests {
		slug, err := testTools.Slugify(e.s)
		if err != nil && !e.errorExpected {
			t.Errorf("%s: error received when none expected: %s", e.name, err.Error())
		}

		if err == nil && e.errorExpected {
			t.Errorf("%s: no error received when an error expected", e.name)
		}

		if !e.errorExpected && slug != e.expected {
			t.Errorf("%s: wrong slug returned; expected %s, but got %s", e.name, e.expected, slug)
		}
	}
}

func TestTools_DownloadStaticFile(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	var testTools Tools

	testTools.DownloadStaticFile(rr, req, "./testdata/pic.jpg", "puppy.jpg")

	res := rr.Result()
	res.Body.Close()

	if res.Header["Content-Length"][0] != "98827" {
		t.Error("wrong content length of ", res.Header["Content-Length"][0])
	}

	if res.Header["Content-Disposition"][0] != "attachment; filename=\"puppy.jpg\"" {
		t.Error("wrong content disposition of ", res.Header["Content-Disposition"][0])
	}

	_, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error(err)
	}
}

var jsonTests = []struct {
	name          string
	json          string
	errorExpected bool
	maxSize       int
	allowUnknown  bool
}{
	{name: "good json", json: `{"foo": "bar"}`, errorExpected: false, maxSize: 1024, allowUnknown: false},
	{name: "badly formatted json", json: `{"foo":}`, errorExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "incorrect type", json: `{"foo": 1}`, errorExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "two json files", json: `{"foo": "1"}{"foo": "2"}`, errorExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "empty body", json: ``, errorExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "syntax error in json", json: `{"foo": 1"`, errorExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "unknown field in json", json: `{"fooo": "1"}`, errorExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "allow unknown field in json", json: `{"fooo": "1"}`, errorExpected: false, maxSize: 1024, allowUnknown: true},
	{name: "missing field name", json: `{jack: "1"}`, errorExpected: true, maxSize: 1024, allowUnknown: true},
	{name: "file too large", json: `{"foo": "bar"}`, errorExpected: true, maxSize: 5, allowUnknown: true},
	{name: "not json", json: `Hello, world!`, errorExpected: true, maxSize: 1024, allowUnknown: true},
}

func TestTools_ReadJSON(t *testing.T) {
	var testTools Tools

	for _, e := range jsonTests {
		// set the max json size
		testTools.MaxJSONSize = e.maxSize

		// allow/disallow unknown fields
		testTools.AllowUnknownFields = e.allowUnknown

		// declare a variable to read the decoded json into
		var decodedJSON struct {
			Foo string `json:"foo"`
		}

		// create a request with the body
		req := httptest.NewRequest("POST", "/", bytes.NewReader([]byte(e.json)))

		// create a recorder
		rr := httptest.NewRecorder()

		err := testTools.ReadJSON(rr, req, &decodedJSON)

		if err == nil && e.errorExpected {
			t.Errorf("%s: error expected, but none received", e.name)
		}

		if err != nil && !e.errorExpected {
			t.Errorf("%s: error not expected, but one received: %s", e.name, err.Error())
		}

		req.Body.Close()
	}
}

func TestTools_WriteJSON(t *testing.T) {
	var testTools Tools

	rr := httptest.NewRecorder()
	payload := JSONResponse{
		Error:   false,
		Message: "foo",
	}

	headers := make(http.Header)
	headers.Add("FOO", "BAR")

	err := testTools.WriteJSON(rr, http.StatusOK, payload, headers)
	if err != nil {
		t.Errorf("failed to write JSON: %v", err)
	}

	if rr.Code != http.StatusOK {
		t.Errorf("expected status code of 200, but got %d", rr.Code)
	}

	res := rr.Result()
	defer res.Body.Close()

	if res.Header["Content-Type"][0] != "application/json" {
		t.Error("wrong content type of ", res.Header["Content-Type"][0])
	}

	if res.Header["Foo"][0] != "BAR" {
		t.Error("wrong header ", res.Header["Foo"][0])
	}

	var response JSONResponse

	body, _ := io.ReadAll(res.Body)
	if err := json.Unmarshal(body, &response); err != nil {
		t.Error("unmarshal error", err)
	}

	if !reflect.DeepEqual(payload, response) {
		t.Errorf("expected body %+v, but got %+v", payload, response)
	}
}

func TestTools_ErrorJSON(t *testing.T) {
	var testTools Tools

	rr := httptest.NewRecorder()
	err := testTools.ErrorJSON(rr, errors.New("some error"), http.StatusServiceUnavailable)
	if err != nil {
		t.Error(err)
	}

	var response JSONResponse

	decorder := json.NewDecoder(rr.Body)
	err = decorder.Decode(&response)
	if err != nil {
		t.Error("received error when decoding JSON", err)
	}

	if !response.Error {
		t.Error("error set to false in JSON, and it should be true")
	}

	if response.Message != "some error" {
		t.Errorf("wrong message received; expected some error, but got %s", response.Message)
	}

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("wrong status code returned; expected 503, but got %d", rr.Code)
	}

}

type RoundTripFunc func(req *http.Request) *http.Response

func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func NewTestClient(fn RoundTripFunc) *http.Client {
	return &http.Client{
		Transport: fn,
	}
}

func TestTools_PushJSONToRemote(t *testing.T) {
	client := NewTestClient(func(req *http.Request) *http.Response {
		// Test Request Parameters
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(bytes.NewBufferString("ok")),
			Header:     make(http.Header),
		}
	})

	var testTools Tools
	var foo struct {
		Bar string `json:"bar"`
	}
	foo.Bar = "bar"

	_, _, err := testTools.PushJSONToRemote("http://example.com/some/path", foo, client)
	if err != nil {
		t.Error("failed to call remote url:", err)
	}
}
