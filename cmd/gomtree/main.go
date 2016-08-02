package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/vbatts/go-mtree"
)

var (
	// flags common with mtree(8)
	flCreate           = flag.Bool("c", false, "create a directory hierarchy spec")
	flFile             = flag.String("f", "", "directory hierarchy spec to validate")
	flPath             = flag.String("p", "", "root path that the hierarchy spec is relative to")
	flAddKeywords      = flag.String("K", "", "Add the specified (delimited by comma or space) keywords to the current set of keywords")
	flUseKeywords      = flag.String("k", "", "Use the specified (delimited by comma or space) keywords as the current set of keywords")
	flUpdateAttributes = flag.Bool("u", false, "Modify the owner, group, permissions and xattrs of files, symbolic links and devices, to match the provided specification. This is not compatible with '-T'.")

	// gomtree specific flags
	flTar              = flag.String("T", "", "use tar archive to create or validate a directory hierarchy spec (\"-\" indicates stdin)")
	flBsdKeywords      = flag.Bool("bsd-keywords", false, "only operate on keywords that are supported by upstream mtree(8)")
	flDebug            = flag.Bool("debug", false, "output debug info to STDERR")
	flListKeywords     = flag.Bool("list-keywords", false, "List the keywords available")
	flListUsedKeywords = flag.Bool("list-used", false, "list all the keywords found in a validation manifest")
	flResultFormat     = flag.String("result-format", "bsd", "output the validation results using the given format (bsd, json, path)")
	flVersion          = flag.Bool("version", false, "display the version of this tool")
)

var formats = map[string]func(*mtree.Result) string{
	// Outputs the errors in the BSD format.
	"bsd": func(r *mtree.Result) string {
		var buffer bytes.Buffer
		for _, fail := range r.Failures {
			fmt.Fprintln(&buffer, fail)
		}
		return buffer.String()
	},

	// Outputs the full result struct in JSON.
	"json": func(r *mtree.Result) string {
		var buffer bytes.Buffer
		if err := json.NewEncoder(&buffer).Encode(r); err != nil {
			panic(err)
		}
		return buffer.String()
	},

	// Outputs only the paths which failed to validate.
	"path": func(r *mtree.Result) string {
		var buffer bytes.Buffer
		for _, fail := range r.Failures {
			fmt.Fprintln(&buffer, fail.Path)
		}
		return buffer.String()
	},
}

func main() {
	flag.Parse()

	if *flDebug {
		os.Setenv("DEBUG", "1")
	}

	// so that defers cleanly exec
	var isErr bool
	defer func() {
		if isErr {
			os.Exit(1)
		}
	}()

	if *flVersion {
		fmt.Printf("%s :: %s\n", os.Args[0], mtree.Version)
		return
	}

	// -list-keywords
	if *flListKeywords {
		fmt.Println("Available keywords:")
		for k := range mtree.KeywordFuncs {
			fmt.Print(" ")
			fmt.Print(k)
			if mtree.Keyword(k).Default() {
				fmt.Print(" (default)")
			}
			if !mtree.Keyword(k).Bsd() {
				fmt.Print(" (not upstream)")
			}
			fmt.Print("\n")
		}
		return
	}

	// --result-format
	formatFunc, ok := formats[*flResultFormat]
	if !ok {
		log.Printf("invalid output format: %s", *flResultFormat)
		isErr = true
		return
	}

	var (
		tmpKeywords     []string
		currentKeywords []string
	)
	// -k <keywords>
	if *flUseKeywords != "" {
		tmpKeywords = splitKeywordsArg(*flUseKeywords)
		if !inSlice("type", tmpKeywords) {
			tmpKeywords = append([]string{"type"}, tmpKeywords...)
		}
	} else {
		if *flTar != "" {
			tmpKeywords = mtree.DefaultTarKeywords[:]
		} else {
			tmpKeywords = mtree.DefaultKeywords[:]
		}
	}

	// -K <keywords>
	if *flAddKeywords != "" {
		for _, kw := range splitKeywordsArg(*flAddKeywords) {
			if !inSlice(kw, tmpKeywords) {
				tmpKeywords = append(tmpKeywords, kw)
			}
		}
	}

	// -bsd-keywords
	if *flBsdKeywords {
		for _, k := range tmpKeywords {
			if mtree.Keyword(k).Bsd() {
				currentKeywords = append(currentKeywords, k)
			} else {
				fmt.Fprintf(os.Stderr, "INFO: ignoring %q as it is not an upstream keyword\n", k)
			}
		}
	} else {
		currentKeywords = tmpKeywords
	}

	// -f <file>
	var dh *mtree.DirectoryHierarchy
	if *flFile != "" && !*flCreate {
		// load the hierarchy, if we're not creating a new spec
		fh, err := os.Open(*flFile)
		if err != nil {
			log.Println(err)
			isErr = true
			return
		}
		dh, err = mtree.ParseSpec(fh)
		fh.Close()
		if err != nil {
			log.Println(err)
			isErr = true
			return
		}
	}

	// -list-used
	if *flListUsedKeywords {
		if *flFile == "" {
			log.Println("no specification provided. please provide a validation manifest")
			defer os.Exit(1)
			isErr = true
			return
		}
		usedKeywords := mtree.CollectUsedKeywords(dh)
		if *flResultFormat == "json" {
			// if they're asking for json, give it to them
			data := map[string][]string{*flFile: usedKeywords}
			buf, err := json.MarshalIndent(data, "", "  ")
			if err != nil {
				defer os.Exit(1)
				isErr = true
				return
			}
			fmt.Println(string(buf))
		} else {
			fmt.Printf("Keywords used in [%s]:\n", *flFile)
			for _, kw := range usedKeywords {
				fmt.Printf(" %s", kw)
				if _, ok := mtree.KeywordFuncs[kw]; !ok {
					fmt.Print(" (unsupported)")
				}
				fmt.Printf("\n")
			}
		}
		return
	}

	// -p <path>
	var rootPath = "."
	if *flPath != "" {
		rootPath = *flPath
	}

	// -u
	// Failing early here. Processing is done below.
	if *flUpdateAttributes && *flTar != "" {
		log.Println("ERROR: -u can not be used with -T")
		isErr = true
		return
	}

	// -T <tar file>
	var tdh *mtree.DirectoryHierarchy
	if *flTar != "" {
		var input io.Reader
		if *flTar == "-" {
			input = os.Stdin
		} else {
			fh, err := os.Open(*flTar)
			if err != nil {
				log.Println(err)
				isErr = true
				return
			}
			defer fh.Close()
			input = fh
		}
		ts := mtree.NewTarStreamer(input, currentKeywords)

		if _, err := io.Copy(ioutil.Discard, ts); err != nil && err != io.EOF {
			log.Println(err)
			isErr = true
			return
		}
		if err := ts.Close(); err != nil {
			log.Println(err)
			isErr = true
			return
		}
		var err error
		tdh, err = ts.Hierarchy()
		if err != nil {
			log.Println(err)
			isErr = true
			return
		}
	}

	// -c
	if *flCreate {
		// create a directory hierarchy
		// with a tar stream
		if tdh != nil {
			tdh.WriteTo(os.Stdout)
		} else {
			// with a root directory
			dh, err := mtree.Walk(rootPath, nil, currentKeywords)
			if err != nil {
				log.Println(err)
				isErr = true
				return
			}
			dh.WriteTo(os.Stdout)
		}
	} else if *flUpdateAttributes && dh != nil {
		// -u
		// this comes before the next case, intentionally.

		// TODO brainstorm where to allow setting of xattrs. Maybe a new flag that allows a comma delimited list of keywords to update?
		updateKeywords := []string{"uid", "gid", "mode"}

		result, err := mtree.Update(rootPath, dh, updateKeywords)
		if err != nil {
			log.Println(err)
			isErr = true
			return
		}

		if result != nil {
			fmt.Printf("%#v\n", result)
		}
	} else if tdh != nil || dh != nil {
		var res *mtree.Result
		var err error
		// else this is a validation
		if *flTar != "" {
			res, err = mtree.TarCheck(tdh, dh, currentKeywords)
		} else {
			res, err = mtree.Check(rootPath, dh, currentKeywords)
		}
		if err != nil {
			log.Println(err)
			isErr = true
			return
		}
		if res != nil && len(res.Failures) > 0 {
			defer os.Exit(1)
			out := formatFunc(res)
			if _, err := os.Stdout.Write([]byte(out)); err != nil {
				log.Println(err)
				isErr = true
				return
			}
		}
		if res != nil {
			if len(res.Extra) > 0 {
				defer os.Exit(1)
				for _, extra := range res.Extra {
					extrapath, err := extra.Path()
					if err != nil {
						log.Println(err)
						isErr = true
						return
					}
					fmt.Printf("%s extra\n", extrapath)
				}
			}
			if len(res.Missing) > 0 {
				defer os.Exit(1)
				for _, missing := range res.Missing {
					missingpath, err := missing.Path()
					if err != nil {
						log.Println(err)
						isErr = true
						return
					}
					fmt.Printf("%s missing\n", missingpath)
				}
			}
		}
	} else {
		log.Println("neither validating or creating a manifest. Please provide additional arguments")
		isErr = true
		defer os.Exit(1)
		return
	}
}

func splitKeywordsArg(str string) []string {
	return strings.Fields(strings.Replace(str, ",", " ", -1))
}

func inSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
