# Scrawl

This project is a fork of [FooSoft/scrawl].

Scrawl is a simple command line tool for downloading files referenced on
websites using
[CSS selectors](https://developer.mozilla.org/en-US/docs/Glossary/CSS_Selector).
This application is not meant to be a replacement for
[curl](http://curl.haxx.se/) or [Wget](https://www.gnu.org/software/wget/), but
rather a precision tool for grabbing files when the context in which they are
presented is known to. This capability is particularly useful when the path of
the desired file is not known but the URL of the website that links to it is
(common for download pages).

## Installation

If you already have the Go environment and toolchain set up, you can get the
latest version by running:

```
$ go install github.com/smrqdt/scrawl@latest
```

Otherwise, you can use the
[pre-built binaries](https://github.com/smrqdt/scrawl/releases) from the project
page.

## Usage

```sh
scrawl [options] <URL> <selector>
```

Executing Scrawl with the `-help` command line argument will trigger online help
to be displayed. Below is a more detailed description of what the parameters do.

- `attr`: The attribute containing the desired download path is specified by
  this argument (e.g. `href` or `src`). If unspecified the elements content will
  be used.
- `dir`: This argument specifies the output directory for downloaded files
  (defaults to the working directory)
- `vebose`: Scrawl will output more details about what it is currently doing
  when this flag is set.
- `overwrite`: Overwrite existing files, if unset scrawl will skip these
  downloads and print a warning.

If the environment variable `DEBUG` is set to any value, debug output will be
enabled.

## Examples

```sh
# download audio element sources
scrawl -dir /tmp -attr src https://tomlehrersongs.com/wernher-von-braun/ "audio source"
# download all pdfs from the download section
scrawl -dir /tmp -attr href https://mikrotik.com/product/rb5009ug_s_in '#downloads a[href$=pdf]'
# download the last XKCD strip (but the higher resolution 2x version)
scrawl -dir /tmp -attr content https://xkcd.com/ 'meta[property="og:image"]'
```
