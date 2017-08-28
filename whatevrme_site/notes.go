package main

import (
	"compress/gzip"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

// NotePad is a bunch of notes stored in a folder, each note is gzipped JSON.
type NotePad struct {
	StoreDir string
}

// NewNoteID returns a unique note ID (urlbase64 of random bytes).
// Should be sufficiently random that we don't have to worry about collisions.
func NewNoteID() string {

	b := make([]byte, 32)

	n, err := rand.Read(b)
	if err != nil {
		panic(err)
	}

	if n != len(b) {
		panic(errors.New("invalid read length from crypto/rand.Read()"))
	}

	return base64.RawURLEncoding.EncodeToString(b)
}

// var NOTE_ALPHABET = `abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789`

var NOTE_ID_PATTERN = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// CheckValidNoteID checks an id for valid chars and length
func CheckValidNoteID(id string) error {

	if !NOTE_ID_PATTERN.MatchString(id) {
		return errors.New("invalid characters in note id")
	}

	if len(id) < 4 {
		return errors.New("note id is too short")
	}

	return nil
}

// NoteIDToPath returns the path-prefixed path for a note id
func NoteIDToPath(id string) (string, error) {

	err := CheckValidNoteID(id)
	if err != nil {
		return "", err
	}

	return id[0:1] + "/" + id[1:2] + "/" + id, nil
}

// NoteExists returns true if a note with this ID exists on the disk
func (np *NotePad) NoteExists(id string) bool {

	p, err := NoteIDToPath(id)
	if err != nil {
		return false
	}

	_, err = os.Stat(filepath.Join(np.StoreDir, p))
	if err != nil {
		return false
	}

	return true
}

// WriteNote writes the JSON marshaled version of note, gzipped, to the appropriate file.
func (np *NotePad) WriteNote(id string, note *Note) error {

	p, err := NoteIDToPath(id)
	if err != nil {
		return err
	}

	dir := filepath.Dir(filepath.Join(np.StoreDir, p))
	os.MkdirAll(dir, 0700)

	// FIXME: what about atomicity - maybe we should be using renames...

	f, err := os.OpenFile(filepath.Join(np.StoreDir, p), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()
	defer gw.Flush()

	enc := json.NewEncoder(gw)
	err = enc.Encode(note)
	if err != nil {
		return err
	}

	return nil

}

func (np *NotePad) ReadNote(id string) (*Note, error) {

	r, err := np.OpenNoteRaw(id)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var ret Note

	gr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}

	dec := json.NewDecoder(gr)
	err = dec.Decode(&ret)
	if err != nil {
		return nil, err
	}

	return &ret, nil
}

func (np *NotePad) OpenNoteRaw(id string) (io.ReadCloser, error) {

	p, err := NoteIDToPath(id)
	if err != nil {
		return nil, err
	}

	return os.Open(filepath.Join(np.StoreDir, p))
}

func (np *NotePad) DeleteNote(id string) error {

	p, err := NoteIDToPath(id)
	if err != nil {
		return err
	}

	return os.Remove(filepath.Join(np.StoreDir, p))
}

// TODO: think about some very basic versioning using a timestamp, if the file was updated
// since the client last read, this case could be detected and an error could go back to
// the client, with an option to override if the user forces.

// Note corresponds to the JSON contents of a note.
type Note struct {
	Timestamp  float64 `json:"timestamp"`   // milliseconds since the epoch
	CipherText []byte  `json:"cipher_text"` // raw cipher text, ends up base64 string in JSON
}

// TODO: Note.CheckValid(), can ensure the note doesn't exceed the max size
