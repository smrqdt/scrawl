package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
)

func scrape(url *url.URL, css, attr string) ([]string, error) {

	buff := &bytes.Buffer{}
	download(url, buff)

	doc, err := goquery.NewDocumentFromReader(buff)
	if err != nil {
		return nil, err
	}

	var assets []string
	doc.Find(css).Each(func(index int, sel *goquery.Selection) {
		asset := sel.Text()
		if len(attr) > 0 {
			asset, _ = sel.Attr(attr)
		}

		asset = strings.TrimSpace(asset)
		if len(asset) > 0 {
			assets = append(assets, asset)
		}
	})

	return assets, nil
}

func download(url *url.URL, w io.Writer) error {
	client := &http.Client{}

	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return fmt.Errorf("could not create request: %w", err)
	}

	req.Header.Add("User-Agent", "scrawl/v1")

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch resource: %w", err)
	}

	defer res.Body.Close()

	io.Copy(w, res.Body)
	return nil
}

func export(path string, r io.Reader) error {
	out, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()

	io.Copy(out, r)
	return nil
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [options] url selector\n", path.Base(os.Args[0]))
	fmt.Fprintln(os.Stderr, "Parameters:")
	flag.PrintDefaults()
}

func main() {
	var (
		attr         = flag.String("attr", "", "attribute to query")
		dir          = flag.String("dir", ".", "output directory")
		verbose      = flag.Bool("verbose", false, "verbose output")
		skipExisting = flag.Bool("skip-existing", true, "skip files that already exist")
	)

	flag.Usage = usage
	flag.Parse()

	if flag.NArg() != 2 {
		flag.Usage()
		os.Exit(2)
	}

	var (
		baseRaw = flag.Arg(0)
		css     = flag.Arg(1)
	)

	base, err := url.Parse(baseRaw)
	if err != nil {
		log.Fatal(err)
	}

	if *verbose {
		log.Printf("scraping page '%s'", baseRaw)
	}

	assetsRaw, err := scrape(base, css, *attr)
	if err != nil {
		log.Fatal(err)
	}

	var wg sync.WaitGroup
	sem := make(chan int, 5)
	for _, assetRaw := range assetsRaw {
		wg.Add(1)
		go func(assetRaw string) {
			defer wg.Done()
			sem <- 1
			defer func() { <-sem }()

			if *verbose {
				log.Printf("parsing asset string '%s'", assetRaw)
			}

			assetURL, err := url.Parse(assetRaw)
			if err != nil {
				log.Fatal(err)
			}

			assetURL = base.ResolveReference(assetURL)

			if *verbose {
				log.Printf("downloading file '%s'", assetURL.String())
			}

			path := filepath.Join(*dir, filepath.Base(assetURL.Path))

			if *skipExisting {
				if _, err := os.Stat(path); err == nil {
					log.Printf("skipping existing file %s", assetURL.String())
					return
				}
			}

			var buff bytes.Buffer
			if err := download(assetURL, &buff); err != nil {
				log.Fatal(err)
			}

			if *verbose {
				log.Printf("writing file '%s'", path)
			}

			if err := export(path, &buff); err != nil {
				log.Fatal(err)
			}
		}(assetRaw)
	}

	wg.Wait()
}
