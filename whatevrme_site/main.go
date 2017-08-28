package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {

	pflag.String("http", ":8880", "HTTP IP:Port to listen on")
	pflag.String("https", ":8843", "HTTPS IP:Port to listen on")
	pflag.String("basedir", "", "Directory of where to find views, static, var, etc. dirs, default is to attempt auto-detection")

	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)

	viper.SetConfigType("yaml")
	viper.SetConfigName("whatevrme_site")
	viper.AddConfigPath("etc")
	viper.AddConfigPath("src/github.com/WhatevrMe/WhatevrMe/etc")
	viper.AddConfigPath("/usr/local/whatevrme/etc")
	viper.AddConfigPath("/etc/whatevrme")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatal(err)
	}

	baseDir := viper.GetString("basedir")

	if baseDir == "" {

		// if basedir is empty try these variations
		tryDirs := []string{
			".",
			"src/github.com/WhatevrMe/WhatevrMe",
			"/usr/local/whatevrme",
		}

		for _, tryDir := range tryDirs {

			trybd, err := filepath.Abs(tryDir)
			if err != nil {
				continue
			}

			st, err := os.Stat(filepath.Join(trybd, "views"))
			if err != nil {
				continue
			}

			if st.IsDir() {
				baseDir = trybd
				break
			}

		}

		if baseDir == "" {
			log.Fatalf("Unable to detect basedir, please specify it on the command line.")
		}

	} else {

		newbd, err := filepath.Abs(baseDir)
		if err != nil {
			log.Fatal(err)
		}
		baseDir = newbd

	}

	log.Printf("basedir = %q", baseDir)

	noteStoreDir := filepath.Join(baseDir, "var/data")
	os.MkdirAll(noteStoreDir, 0755)

	notePad := &NotePad{
		StoreDir: noteStoreDir,
	}

	webServer := &WebServer{
		NotePad:    notePad,
		ViewsFS:    http.Dir(filepath.Join(baseDir, "views")),
		IncludesFS: http.Dir(filepath.Join(baseDir, "includes")),
		StaticFS:   http.Dir(filepath.Join(baseDir, "static")),
	}

	// TODO: HTTPS support

	listenHttp := viper.GetString("http")
	log.Printf("Starting HTTP Listener at %q", listenHttp)
	log.Fatal(http.ListenAndServe(listenHttp, webServer))

}
