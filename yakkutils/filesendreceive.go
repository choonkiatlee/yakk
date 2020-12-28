package yakkutils

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/cavaliercoder/grab"
	"github.com/flowchartsman/retry"
	"github.com/rs/zerolog/log"
)

// const(
// 	YAKK_FILENAME
// )

func ServeFile(filename string, port string, wg *sync.WaitGroup) *http.Server {
	serveFileHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		http.ServeFile(w, r, filename)
	}

	srv := &http.Server{Addr: fmt.Sprintf(":%s", port)}
	http.HandleFunc(fmt.Sprintf("/yakkfilesendreceive"), serveFileHandler)

	go func() {
		defer wg.Done() // used to let main know we are done
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal().Msgf("Listen and serve failure: %v", err)
		}
	}()
	return srv
}

func ReceiveFile(filename string, port string) {
	client := grab.NewClient()
	req, err := grab.NewRequest("", fmt.Sprintf("%s:%s/yakkfilesendreceive%s", "http://localhost", port, filename))
	if err != nil {
		panic(err)
	}

	retrier := retry.NewRetrier(5, 100*time.Millisecond, 5*time.Second)

	// start download
	retrier.Run(func() error { return downloadReq(client, req) })
}

func downloadReq(client *grab.Client, req *grab.Request) error {
	log.Info().Msgf("Downloading %v...\n", req.URL())
	resp := client.Do(req)
	// fmt.Printf("  %v\n", resp.HTTPResponse.Status)

	// start UI loop
	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()

Loop:
	for {
		select {
		case <-t.C:
			log.Info().Msgf("  transferred %v / %v bytes (%.2f%%)\n",
				resp.BytesComplete(),
				resp.Size,
				100*resp.Progress())

		case <-resp.Done:
			// download is complete
			break Loop
		}
	}

	// check for errors
	if err := resp.Err(); err != nil {
		log.Error().Msgf("Download failed: %v\n", err)
		return err
	}

	fmt.Printf("Download saved to ./%v \n", resp.Filename)
	return nil

	// Output:
	// Downloading http://www.golang-book.com/public/pdf/gobook.pdf...
	//   200 OK
	//   transferred 42970 / 2893557 bytes (1.49%)
	//   transferred 1207474 / 2893557 bytes (41.73%)
	//   transferred 2758210 / 2893557 bytes (95.32%)
	// Download saved to ./gobook.pdf
}
