package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var _, debug = os.LookupEnv("DEBUG")
var client = &http.Client{}

func scrape(url *url.URL, css, attr string) ([]string, error) {

	buff := &bytes.Buffer{}
	err := download(url, buff)
	if err != nil {
		return nil, fmt.Errorf("could not download base url: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(buff)
	if err != nil {
		return nil, fmt.Errorf("could not parse website: %w", err)
	}

	var assets []string
	doc.Find(css).Each(func(index int, sel *goquery.Selection) {
		selHTML, _ := goquery.OuterHtml(sel)
		target := sel.Text()
		if attr != "" {
			var ok bool
			target, ok = sel.Attr(attr)
			if !ok {
				log.Debug().
					Str("selection", selHTML).
					Str("attr", attr).
					Msg("attribute not found on element")
			}
		}

		target = strings.TrimSpace(target)
		if target == "" {

			log.Debug().
				Str("selection", selHTML).
				Str("attr", attr).
				Msg("target is empty")
		}
		assets = append(assets, target)
	})

	return assets, nil
}

func download(url *url.URL, w io.Writer) error {
	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return fmt.Errorf("could not create request: %w", err)
	}

	req.Header.Add("User-Agent", "scrawl/v1")

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch resource: %w", err)
	}

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("received HTTP status %v", res.Status)
	}

	defer res.Body.Close()

	io.Copy(w, res.Body)
	return nil
}

func export(path string, r io.Reader) error {
	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("could not create file: %w", err)
	}
	defer out.Close()

	io.Copy(out, r)
	return nil
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [options] <url> <selector>\n", path.Base(os.Args[0]))
	fmt.Fprintln(os.Stderr, "Options:")
	flag.PrintDefaults()
}

func main() {
	var (
		attr      = flag.String("attr", "", "attribute to query")
		dir       = flag.String("dir", ".", "output directory")
		verbose   = flag.Bool("verbose", false, "verbose output")
		overwrite = flag.Bool("overwrite", false, "overwrite files that already exist (will be skipped otherwise)")
	)

	flag.Usage = usage
	flag.Parse()

	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	if *verbose {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	if flag.NArg() != 2 {
		flag.Usage()
		log.Fatal().Strs("args", os.Args[1:]).Msg("missing arguments")
	}

	baseRaw := flag.Arg(0)
	css := flag.Arg(1)

	base, err := url.Parse(baseRaw)
	if err != nil {
		log.Fatal().Err(err).Str("url", baseRaw).Msg("could not parse base URL")
	}

	log.Info().
		Str("url", baseRaw).
		Msg("scraping page")

	assetsRaw, err := scrape(base, css, *attr)
	if err != nil {
		log.Fatal().Err(err).
			Str("css", css).
			Str("url", base.String()).
			Str("attr", *attr).
			Msg("could not scrape assets")
	}

	if len(assetsRaw) == 0 {
		log.WithLevel(zerolog.FatalLevel).Msg("no assets found")
		os.Exit(2)
	}

	log.Info().Int("results", len(assetsRaw)).Msg("assets found")
	var wg sync.WaitGroup
	sem := make(chan int, 5)
	for jobID, assetRaw := range assetsRaw {
		wg.Add(1)
		log.Debug().
			Int("job", jobID).
			Str("target", assetRaw).
			Msg("queuing job")
		go func(jobID int, assetRaw string) {
			defer wg.Done()
			sem <- 1
			defer func() { <-sem }()

			log.Debug().
				Int("job", jobID).
				Str("target", assetRaw).
				Msg("starting download job")

			assetURL, err := url.Parse(assetRaw)
			if err != nil {
				log.Fatal().Err(err).
					Int("job", jobID).
					Str("target", assetRaw).
					Msg("could not parse asset target into URL")
			}

			assetURL = base.ResolveReference(assetURL)
			path := filepath.Join(*dir, filepath.Base(assetURL.Path))

			if _, err := os.Stat(path); err == nil {
				if !*overwrite {
					log.Warn().
						Int("job", jobID).
						Str("url", assetURL.String()).
						Msg("skipping existing file")
					return
				}
				log.Warn().
					Int("job", jobID).
					Str("url", assetURL.String()).
					Msg("overwriting existing file")
			}

			log.Info().
				Int("job", jobID).
				Str("url", assetURL.String()).
				Msg("downloading file")

			var buff bytes.Buffer
			if err := download(assetURL, &buff); err != nil {
				log.Error().Err(err).
					Int("job", jobID).
					Str("url", assetURL.String()).
					Msg("could not download asset")
				return
			}

			log.Debug().
				Int("job", jobID).
				Str("filename", path).
				Msg("writing file")

			if err := export(path, &buff); err != nil {
				log.Fatal().Err(err).
					Int("job", jobID).
					Str("filename", path).
					Msg("could not write file")
				return
			}

			log.Info().Int("job", jobID).
				Str("path", path).
				Str("url", assetURL.String()).
				Msg("successful download")

		}(jobID, assetRaw)
	}

	wg.Wait()
}
