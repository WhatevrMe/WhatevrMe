package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	mathrand "math/rand"
	"mime"
	"net/http"
	"path"
	"runtime"
	"strings"
	"text/template/parse"
	"time"
)

type WebServer struct {
	NotePad    *NotePad
	ViewsFS    http.FileSystem
	IncludesFS http.FileSystem
	StaticFS   http.FileSystem
}

func (ws *WebServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	p := path.Clean("/" + r.URL.Path)

	ctx := r.Context()

	// if this looks like either a note id or short id
	if len(p) >= 5 && !strings.Contains(p, ".") && !strings.Contains(p[1:], "/") {

		id := p[1:]
		if ws.NotePad.NoteExists(id) {

			ctx = context.WithValue(ctx, "note-id", id)
			p = "/note.html"

		}

		// TODO: check note, then internally rewrite the URL here and modify the context
		// http.Error(w, "not implemented", 500)
		// return
	}

	// api calls go in handleAPICalls
	if strings.HasPrefix(p, "/api/") {
		ws.handleAPICalls(w, r)
		return
	}

	// path hack(s)
	if p == "/" {
		p = "/index.html"
	}

	// check for view template
	f, err := ws.ViewsFS.Open(p)
	if err == nil {
		defer f.Close()

		st, err := f.Stat()
		if err != nil {
			HTTPError(w, r, err, "internal error", 500)
			return
		}
		// ignore dirs
		if !st.IsDir() {

			b, err := ioutil.ReadAll(f)
			if err != nil {
				HTTPError(w, r, err, "internal error", 500)
				return
			}

			t, err := template.New("__page__").Parse(string(b))
			if err != nil {
				HTTPError(w, r, err, "internal error", 500)
				return
			}

			err = TmplIncludeAll(ws.IncludesFS, t)
			if err != nil {
				HTTPError(w, r, err, "internal error", 500)
				return
			}

			w.Header().Set("content-type", "text/html; charset=UTF-8")

			err = t.ExecuteTemplate(w, "__page__", ctx)
			if err != nil {
				HTTPError(w, r, err, "internal error", 500)
				return
			}

			return
		}
	}

	// check for static file
	f, err = ws.StaticFS.Open(p)
	if err == nil {
		defer f.Close()
		st, err := f.Stat()
		if err != nil {
			HTTPError(w, r, err, "internal error", 500)
			return
		}
		// ignore dirs
		if !st.IsDir() {

			// give it the old college try on the mime type detection
			ct := GuessMIMEType(p)
			if ct != "" {
				w.Header().Set("content-type", ct)
			}

			http.ServeContent(w, r, p, st.ModTime(), f)
			return
		}
	}

	// otherwise 404
	http.NotFound(w, r)
}

func (ws *WebServer) handleAPICalls(w http.ResponseWriter, r *http.Request) {

	p := path.Clean("/" + r.URL.Path)

	pparts := strings.Split(strings.Trim(p, "/"), "/")

	switch {

	case len(pparts) == 3 && pparts[0] == "api" && pparts[1] == "note" && r.Method == "GET": // get

		id := pparts[2]

		nr, err := ws.NotePad.OpenNoteRaw(id)
		if err != nil {
			HTTPError(w, r, err, "error getting note", 404)
			break
		}
		defer nr.Close()

		w.Header().Set("content-type", "application/json")
		// FIXME: technically we should only be doing this if the client sends gzip in accept-encoding
		w.Header().Set("content-encoding", "gzip")

		_, err = io.Copy(w, nr)
		if err != nil {
			HTTPError(w, r, err, "internal error", 500)
			break
		}

	case len(pparts) == 2 && pparts[0] == "api" && pparts[1] == "note" && r.Method == "POST": // create

		defer r.Body.Close()
		dec := json.NewDecoder(r.Body)

		var note Note
		err := dec.Decode(&note)
		if err != nil {
			HTTPError(w, r, err, "error decoding", 500)
			break
		}

		id := NewNoteID()
		err = ws.NotePad.WriteNote(id, &note)
		if err != nil {
			HTTPError(w, r, err, "error writing", 500)
			break
		}

		w.Header().Set("location", "/api/note/"+id)
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(201)
		w.Write([]byte(`{"id":"` + id + `"}`))

	case len(pparts) == 3 && pparts[0] == "api" && pparts[1] == "note" && (r.Method == "POST" || r.Method == "PUT"): // update

		id := pparts[2]

		defer r.Body.Close()
		dec := json.NewDecoder(r.Body)

		var note Note
		err := dec.Decode(&note)
		if err != nil {
			HTTPError(w, r, err, "error decoding", 500)
			break
		}

		err = ws.NotePad.WriteNote(id, &note)
		if err != nil {
			HTTPError(w, r, err, "error writing", 500)
			break
		}

		w.Header().Set("content-type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"id":"` + id + `"}`))

	case len(pparts) == 3 && pparts[0] == "api" && pparts[1] == "note" && r.Method == "DELETE": // delete

		id := pparts[2]

		if !ws.NotePad.NoteExists(id) {
			http.NotFound(w, r)
			return
		}

		err := ws.NotePad.DeleteNote(id)
		if err != nil {
			HTTPError(w, r, err, "internal error", 500)
			break
		}

		w.WriteHeader(204)

	default:
		http.NotFound(w, r)
	}

}

func TmplIncludeAll(fs http.FileSystem, t *template.Template) error {

	tlist := t.Templates()
	for _, et := range tlist {
		if et != nil && et.Tree != nil && et.Tree.Root != nil {
			err := TmplIncludeNode(fs, et, et.Tree.Root)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func TmplIncludeNode(fs http.FileSystem, t *template.Template, node parse.Node) error {

	if node == nil {
		return nil
	}

	switch node := node.(type) {

	case *parse.TemplateNode:
		if node == nil {
			return nil
		}

		// if template is already defined, do nothing
		tlist := t.Templates()
		for _, et := range tlist {
			if node.Name == et.Name() {
				return nil
			}
		}

		t2 := t.New(node.Name)

		f, err := fs.Open(node.Name)
		if err != nil {
			return err
		}
		defer f.Close()

		b, err := ioutil.ReadAll(f)
		if err != nil {
			return err
		}

		_, err = t2.Parse(string(b))
		if err != nil {
			return err
		}

		// start over again, will stop recursing when there are no more templates to include
		return TmplIncludeAll(fs, t)

	case *parse.ListNode:

		if node == nil {
			return nil
		}

		for _, node := range node.Nodes {
			err := TmplIncludeNode(fs, t, node)
			if err != nil {
				return err
			}
		}

	case *parse.IfNode:
		if err := TmplIncludeNode(fs, t, node.BranchNode.List); err != nil {
			return err
		}
		if err := TmplIncludeNode(fs, t, node.BranchNode.ElseList); err != nil {
			return err
		}

	case *parse.RangeNode:
		if err := TmplIncludeNode(fs, t, node.BranchNode.List); err != nil {
			return err
		}
		if err := TmplIncludeNode(fs, t, node.BranchNode.ElseList); err != nil {
			return err
		}

	case *parse.WithNode:
		if err := TmplIncludeNode(fs, t, node.BranchNode.List); err != nil {
			return err
		}
		if err := TmplIncludeNode(fs, t, node.BranchNode.ElseList); err != nil {
			return err
		}

	}

	return nil
}

// Reads and reports an http error - does not expose anything to the outside
// world except a unique ID, which can be matched up with the appropriate log
// statement which has the details.
func HTTPError(w http.ResponseWriter, r *http.Request, err error, publicMessage string, code int) error {

	if err == nil {
		err = errors.New(publicMessage)
	}

	id := fmt.Sprintf("%x", time.Now().Unix()^mathrand.Int63())

	_, file, line, _ := runtime.Caller(1)

	w.Header().Set("x-error-id", id) // make a way for the client to programatically extract the error id
	http.Error(w, fmt.Sprintf("Error serving request (id=%q) %s", id, publicMessage), code)

	log.Printf("HTTPError: (id=%q) %s:%v | %v", id, file, line, err)

	return err
}

// GuessMIMEType is a thin wrapper around mime.TypeByExtension(),
// but with some common sense defaults that are sometimes different/wrong
// on different platforms for no good reason.
func GuessMIMEType(p string) string {

	pext := path.Ext(p)

	switch pext {
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".html":
		return "text/html"
	}

	return mime.TypeByExtension(pext)

}
