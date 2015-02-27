package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

import (
	"github.com/AdRoll/goamz/aws"
	"github.com/AdRoll/goamz/s3"
	"github.com/AdRoll/goamz/sqs"
	"github.com/joho/godotenv"
)

const s3_folder = "models/"

var sqsRegion aws.Region = aws.USEast

type SlicingJob struct {
	Id         int    `json: "id"`
	SlicerName string `json: "slicerName"`
	Filename   string `json: "filename"`
	//	Filetype   string `json: "filetype"`
	Status string `json: "status"`
}

func slice(job *SlicingJob, inFilename, tempDir string) (string, error) {
	// Process job to find slic3r path
	slicerPath, exists := map[string]string{
		"slic3r": "bin/Slic3r/bin/slic3r",
	}[job.SlicerName]
	if exists == false {
		log.Fatal("Slicer for SlicerName " + job.SlicerName + " not found.")
	}

	// Get paths and generate command
	basedir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}
	absSlicerPath := filepath.Join(basedir, slicerPath)
	aipath := filepath.Join(tempDir, inFilename)
	aopath := filepath.Join(tempDir, "output-"+strconv.Itoa(job.Id)+".gcode")
	c := exec.Command(absSlicerPath, aipath, "-o", aopath)
	fmt.Printf("Executing ", c.Args, "\n")
	c.Stdout = os.Stdout

	// Run!
	if err := c.Run(); err != nil {
		fmt.Printf("%T\n", err)
		log.Fatalf("ERR!", err)
	}

	return aopath, nil
}

// Loads slicing job from SQS, fetches stl file, slices it, saves gcode to S3,
// puts info back on queue
func loadAndProcess(bucket *s3.Bucket, queueIn *sqs.Queue, queueOut *sqs.Queue) {

	log.Printf("loadAndProcess called with s3 bucket %s, queue in %s and out %s",
		bucket.Name, queueIn.Name, queueOut.Name)

	params := map[string]string{
		"MaxNumberOfMessages": "1",
		"VisibilityTimeout":   "100",
	}
	resp, err := queueIn.ReceiveMessageWithParameters(params)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Num of messages received: %d", len(resp.Messages))
	if len(resp.Messages) == 0 {
		log.Printf("Returning.")
		return
	} else {
		log.Printf("Message found on slicer queue:", resp.Messages[0])
		// Pull from queue so others don't see it
		_, err = queueIn.DeleteMessage(&resp.Messages[0])
		if err != nil {
			log.Fatal(err)
		}
	}

	// Parse SlicingJob from json
	job := &SlicingJob{}
	if err := json.Unmarshal([]byte(resp.Messages[0].Body), &job); err != nil {
		log.Fatal("Error parsing message json", err)
	}
	fmt.Printf("Parsed job: %+v\n", job)

	// Get file from S3
	if len(job.Filename) == 0 {
		log.Fatal("Required attribute Filename not found in job")
	}
	filename := strconv.Itoa(job.Id) + "-" + job.Filename
	path := filepath.Join(s3_folder, filename)
	bytes, err := bucket.Get(path)
	if err != nil {
		log.Fatal("Failed to find s3 file at path "+path, err)
	}

	// Write to tmp
	tmpPath := "/tmp/" + filename
	if err := ioutil.WriteFile(tmpPath, bytes, 0644); err != nil {
		log.Fatal("Failed to save to "+tmpPath, err)
	}

	aopath, _ := slice(job, filename, "/tmp")

	// Open generated gcode
	// Save gcode to de cloud
	generated, err := ioutil.ReadFile(aopath)
	if err != nil {
		log.Fatal("Failed to read generated gcode on path "+aopath, err)
	} else {
		log.Printf("Generated gcode file was read at %s", aopath)
	}
	outs3fn := "gcode/" + strconv.Itoa(job.Id) + ".gcode"
	options := s3.Options{}
	if err := bucket.Put(outs3fn, generated, "", "", options); err != nil {
		log.Fatal("Failed to put sliced model in s3 bucket", err)
	} else {
		log.Printf("Gcode file %s added to s3 bucket at %s\n", aopath, outs3fn)
	}

	// Create message saying gcode was sliced
	job.Status = "done"
	b, err := json.Marshal(job)
	if err != nil {
		log.Fatalf("Failed to convert job %+v to json", job, err)
	}
	if resp, err := queueOut.SendMessage(string(b)); err != nil {
		log.Fatal("Failed to send message to queueOut", err)
	} else {
		log.Printf("Job added to processed queue", resp)
	}

}

func main() {
	// Setup env
	err := godotenv.Load("../.env")
	if err != nil {
		log.Printf("Error! loading .env file", err)
	}

	auth, err := aws.EnvAuth()
	if err != nil {
		log.Fatal(err)
	}

	bucketName := os.Getenv("S3_BUCKET")
	if len(bucketName) == 0 {
		log.Fatal("S3_BUCKET env variable required but not found")
	}
	queueNameIn := os.Getenv("SQS_QUEUE_IN")
	if len(queueNameIn) == 0 {
		log.Fatal("SQS_QUEUE_IN env variable required but not found")
	}
	queueNameOut := os.Getenv("SQS_QUEUE_OUT")
	if len(queueNameOut) == 0 {
		log.Fatal("SQS_QUEUE_OUT env variable required but not found")
	}

	// Get s3 bucket
	bucket := s3.New(auth, aws.SAEast).Bucket(bucketName)

	// Get sqs queues
	sqsClient := sqs.New(auth, sqsRegion)
	queueIn, err := sqsClient.GetQueue(queueNameIn)
	if err != nil {
		log.Fatal(err)
	}
	queueOut, err := sqsClient.GetQueue(queueNameOut)
	if err != nil {
		log.Fatal(err)
	}

	job := &SlicingJob{
		Id:         123,
		SlicerName: "slic3r",
		Filename:   "me.stl",
		Status:     "pending",
	}
	messageJson, err := json.Marshal(job)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(messageJson), job)
	queueIn.SendMessage(string(messageJson))

	loadAndProcess(bucket, queueIn, queueOut)
}
