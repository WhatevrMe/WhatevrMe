package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var wsTestKeepTmp = flag.Bool("ws-test-keep-tmp", false, "Keep the webservertest directory(s) instead of deleting them")

func TestWebServerAPI(t *testing.T) {

	tmpDir, err := ioutil.TempDir("", "TestWebServerAPI")
	if err != nil {
		t.Fatal(err)
	}
	if !*wsTestKeepTmp {
		defer func() {
			t.Logf("Removing temp dir: %q", tmpDir)
			os.RemoveAll(tmpDir)
		}()
	}

	dataDir := filepath.Join(tmpDir, "var/data")
	os.MkdirAll(dataDir, 0755)

	t.Logf("TestWebServerAPI data dir: %q", dataDir)

	notePad := &NotePad{
		StoreDir: dataDir,
	}

	ws := &WebServer{
		NotePad:    notePad,
		ViewsFS:    http.Dir(filepath.Join(dataDir, "views")),
		IncludesFS: http.Dir(filepath.Join(dataDir, "includes")),
		StaticFS:   http.Dir(filepath.Join(dataDir, "static")),
	}

	// try creating some notes and manipulating them

	tsrv := httptest.NewServer(ws)
	defer tsrv.Close()

	client := tsrv.Client()

	note := &Note{
		Timestamp: float64(time.Now().UnixNano() / int64(time.Millisecond)),
	}

	b, err := json.Marshal(note)
	if err != nil {
		t.Fatal(err)
	}

	// create a note
	req, err := http.NewRequest("POST", tsrv.URL+"/api/note", bytes.NewBuffer(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("content-type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	b, _ = httputil.DumpResponse(res, true)
	t.Logf("create response:\n%s", b)

	if res.StatusCode != 201 {
		t.Fatalf("Wrong status code, expected 201")
	}

	loc := res.Header.Get("location")
	locParts := strings.Split(loc, "/")
	id := locParts[len(locParts)-1]

	// now try getting it
	req, err = http.NewRequest("GET", tsrv.URL+"/api/note/"+id, nil)
	if err != nil {
		t.Fatal(err)
	}

	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	b, _ = httputil.DumpResponse(res, true)
	t.Logf("get response:\n%s", b)

	if res.StatusCode != 200 {
		t.Fatalf("Wrong status code, expected 200")
	}

	// update it
	note.Timestamp = float64(time.Now().UnixNano() / int64(time.Millisecond))
	b, err = json.Marshal(note)
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", tsrv.URL+"/api/note/"+id, bytes.NewBuffer(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("content-type", "application/json")

	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	b, _ = httputil.DumpResponse(res, true)
	t.Logf("update response:\n%s", b)

	if res.StatusCode != 200 {
		t.Fatalf("Wrong status code, expected 200")
	}

	// delete it
	req, err = http.NewRequest("DELETE", tsrv.URL+"/api/note/"+id, nil)
	if err != nil {
		t.Fatal(err)
	}

	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	b, _ = httputil.DumpResponse(res, true)
	t.Logf("delete response:\n%s", b)

	if res.StatusCode != 204 {
		t.Fatalf("Wrong status code, expected 204")
	}

	// now try getting it again
	req, err = http.NewRequest("GET", tsrv.URL+"/api/note/"+id, nil)
	if err != nil {
		t.Fatal(err)
	}

	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	b, _ = httputil.DumpResponse(res, true)
	t.Logf("get response (after delete):\n%s", b)

	if res.StatusCode != 404 {
		t.Fatalf("Wrong status code, expected 404")
	}

}
