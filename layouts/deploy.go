package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	deployTracer = otel.Tracer("deploy")
	deployMeter  = otel.Meter("deploy")
	deployTime   metric.Int64Counter
	localPath    string
	bucket       string
	prefix       string
)

func init() {
	var err error
	deployTime, err = deployMeter.Int64Counter("deploy.time",
		metric.WithDescription("The seconds for pushing to s3"),
		metric.WithUnit("{ti}"))
	if err != nil {
		panic(err)
	}
	prefix = "www/status/"
}

func deploy(w http.ResponseWriter, r *http.Request) {

	ctx, span := deployTracer.Start(r.Context(), "deploy")
	defer span.End()

	startTime := time.Now()

	uploadFolder(appConfig.PublicFolder)

	duration := time.Since(startTime)

	// Add the custom attribute to the span and counter.
	deployValueAttr := attribute.Int("deploy.time", int(duration.Seconds()))
	span.SetAttributes(deployValueAttr)
	deployTime.Add(ctx, 1, metric.WithAttributes(deployValueAttr))
}

func uploadFolder(publicFolder string) {

	walker := make(fileWalk)
	go func() {
		// Gather the files to upload by walking the path recursively
		if err := filepath.Walk(publicFolder, walker.Walk); err != nil {
			log.Println("Walk failed:", err)
		}
		close(walker)
	}()

	// files := &filesWalked{
	// 	files: make([]string, 0),
	// }

	// if err := filepath.Walk(publicFolder, files.Walk); err != nil {
	// 	log.Println("walk failed:", err)
	// }
	// The session the S3 Uploader will use
	sess := session.Must(session.NewSession())

	// S3 service client the Upload manager will use.
	s3Svc := s3.New(sess)

	// Create an uploader with S3 client and default options
	// uploader := s3manager.NewUploaderWithClient(s3Svc)

	// Create an uploader with S3 client and custom options
	uploader := s3manager.NewUploaderWithClient(s3Svc, func(u *s3manager.Uploader) {
		u.PartSize = 64 * 1024 * 1024 // 64MB per part
	})

	// cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-east-1"))
	// if err != nil {
	// 	log.Fatalln("error:", err)
	// }

	// For each file found walking, upload it to Amazon S3
	// uploader := manager.NewUploader(s3.NewFromConfig(cfg))
	for path := range walker {
		// log.Println(path)
		rel, err := filepath.Rel(localPath, path)
		if err != nil {
			log.Println("Unable to get relative path:", path, err)
		}
		contentTypeFileDescriptor, err := os.Open(path)
		if err != nil {
			log.Println("Failed opening file", path, err)
			continue
		}
		defer contentTypeFileDescriptor.Close()

		// Helper function to do Content-Type Handling
		contentType, err := getContentType(contentTypeFileDescriptor)
		if err != nil {
			log.Println("Error reading file metadata", contentTypeFileDescriptor.Name())
		}

		uploadFileDescriptor, err := os.Open(path)
		if err != nil {
			log.Println("Failed opening file", path, err)
			continue
		}
		defer uploadFileDescriptor.Close()

		// TODO: Need to miror bucket config with how code deploys file paths
		_, err = uploader.Upload(&s3manager.UploadInput{
			Bucket:      aws.String(appConfig.Defaults.Bucket),
			Key:         aws.String(filepath.Join(prefix, rel)),
			ContentType: aws.String(contentType),
			Body:        uploadFileDescriptor,
		})

		if err != nil {
			log.Println("Failed to upload", path, err)
		}
		// log.Println("Uploaded", path, result.Location)
	}
}

func getContentType(file *os.File) (string, error) {

	buf := make([]byte, 512)

	_, err := file.Read(buf)
	if err != nil {
		log.Printf("Unable to read content type %v", file.Name())
		return "", err
	}
	if strings.HasSuffix(file.Name(), ".css") {
		return "text/css", nil
	}
	contentType := http.DetectContentType(buf)
	return contentType, nil

}

type fileWalk chan string

func (f fileWalk) Walk(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	if !info.IsDir() {
		f <- path
	}
	return nil
}

// type filesWalked struct {
// 	files []string
// }

// // synchronous changes
// func (f *filesWalked) Walk(path string, info os.FileInfo, err error) error {
// 	if err != nil {
// 		return err
// 	}

// 	if !info.IsDir() {
// 		f.files = append(f.files, path)
// 	}

// 	return nil
// }
