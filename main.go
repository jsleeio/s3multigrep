package main

import (
	"bufio"
	"compress/bzip2"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// AppContext encapsulates global app config
type AppContext struct {
	Region *string
	Bucket *string
	Prefix *string
	S3     *s3.S3
}

// NewAppContext initialises a global app config object
func NewAppContext() *AppContext {
	context := &AppContext{
		Region: flag.String("region", "us-west-2", "AWS region to operate in"),
		Bucket: flag.String("bucket", "", "Name of S3 bucket to operate in"),
		Prefix: flag.String("prefix", "", "Bucket object base prefix"),
	}
	sess, _ := session.NewSession(&aws.Config{Region: aws.String(*context.Region)})
	context.S3 = s3.New(sess)
	return context
}

// MatchJob encapsulates data for a search operation
type MatchJob struct {
	Context      *AppContext
	NameMatch    *regexp.Regexp
	ContentMatch *regexp.Regexp
	ShowKeys     bool
}

// NewMatchJob initialises a MatchJob object and compiles regexes
func NewMatchJob(ctx *AppContext, nmatch, cmatch string) *MatchJob {
	mj := &MatchJob{
		Context:      ctx,
		NameMatch:    regexp.MustCompile(nmatch),
		ContentMatch: regexp.MustCompile(cmatch),
		ShowKeys:     false,
	}
	return mj
}

// SetShowKeys alters the value of the ShowKeys option
func (mj *MatchJob) SetShowKeys(sk *bool) {
	mj.ShowKeys = *sk
}

// ListObjectsWithCallback lists all objects in a bucket and invokes a
// callback for each page
func (mj *MatchJob) ListObjectsWithCallback(fn func(*s3.ListObjectsV2Output, bool) bool) error {
	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(*mj.Context.Bucket),
		MaxKeys: aws.Int64(100),
		Prefix:  mj.Context.Prefix,
	}
	return mj.Context.S3.ListObjectsV2Pages(input, fn)
}

// JustListNameMatches does exactly that; no content matching is performed
func (mj *MatchJob) JustListNameMatches() {
	err := mj.ListObjectsWithCallback(func(page *s3.ListObjectsV2Output, last bool) bool {
		for _, obj := range page.Contents {
			if mj.NameMatch.MatchString(*obj.Key) {
				fmt.Fprintf(os.Stderr, "object key matched: %v\n", *obj.Key)
			}
		}
		return last
	})
	if err != nil {
		panic(err)
	}
}

// GetObject wraps S3.GetObject with local context
func (mj *MatchJob) GetObject(key string) (*s3.GetObjectOutput, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(*mj.Context.Bucket),
		Key:    aws.String(key),
	}
	return mj.Context.S3.GetObject(input)
}

// TransparentExpandingReader creates a Reader that transparently decompresses based
// on filename
func TransparentExpandingReader(key string, source io.ReadCloser) io.Reader {
	ext := path.Ext(key)
	var reader io.Reader
	var err error
	switch {
	case ext == ".gz":
		reader, err = gzip.NewReader(source)
		if err != nil {
			panic(err)
		}
	case ext == ".bz2":
		reader = bzip2.NewReader(source)
	default:
		reader = bufio.NewReader(source)
	}
	return reader
}

// ListContentMatches ...
func (mj *MatchJob) ListContentMatches() {
	totalMatches := 0
	objchan := make(chan *s3.GetObjectOutput, 5000)
	errchan := make(chan string, 5000)
	err := mj.ListObjectsWithCallback(func(page *s3.ListObjectsV2Output, last bool) bool {
		var wg sync.WaitGroup
		for _, obj := range page.Contents {
			if mj.NameMatch.MatchString(*obj.Key) {
				wg.Add(1)
				go func(key string) {
					defer wg.Done()
					obj, err := mj.GetObject(key)
					if err != nil {
						errchan <- key
					} else {
						reader := TransparentExpandingReader(key, obj.Body)
						scanner := bufio.NewScanner(reader)
						matches := 0
						for scanner.Scan() {
							text := scanner.Text()
							if mj.ContentMatch.MatchString(text) {
								if mj.ShowKeys {
									fmt.Printf("%s:%s\n", key, text)
								} else {
									fmt.Println(text)
								}
								matches++
							}
						}
						fmt.Fprintf(os.Stderr, "%s: %d matches\n", key, matches)
						totalMatches += matches
						objchan <- obj
					}
				}(*obj.Key)
			}
		}
		go func() {
			wg.Wait()
		}()
		return true
	})
	if err != nil {
		panic(err)
	}
	close(objchan)
	close(errchan)
	var totalLength int64
	objs := 0
	for obj := range objchan {
		totalLength += *obj.ContentLength
		objs++
	}
	fmt.Fprintf(os.Stderr, "searched %d MB logs in %d objects and found %d matches\n",
		totalLength/1048576, objs, totalMatches)
}

func main() {
	context := NewAppContext()
	showkeys := flag.Bool("show-keys", false, "Include S3 keys with matching lines, like traditional grep")
	keymatch := flag.String("key-match", "", "String match on S3 object key")
	contentmatch := flag.String("content-match", "", "String match on S3 object key")
	flag.Parse()
	mj := NewMatchJob(context, *keymatch, *contentmatch)
	mj.SetShowKeys(showkeys)
	mj.ListContentMatches()
}
