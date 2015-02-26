
package main

import (
	"fmt"
	"time"
	"log"
	"os/exec"
	"path/filepath"
	"os"
)

import (
	"github.com/AdRoll/goamz/s3"
	"github.com/AdRoll/goamz/aws"
	"github.com/AdRoll/goamz/sqs"
	"github.com/joho/godotenv"
)


var region aws.Region = aws.USEast


func receiveMessages(queue * sqs.Queue, in <-chan string) {
	go func() {
		for {
			select {
			case msg := <-in:
				log.Println("Receive message from channel", msg)
				mr, _ := queue.ReceiveMessage(1)
				for i := range mr.Messages {
					fmt.Println("Damn", mr.Messages[i])
				}
			}
		}
	}()
}

func sendMessages(queue * sqs.Queue, out chan<- string) {
	ticker := time.NewTicker(5 * time.Second)
	quit := make(chan struct{})
	go func () {
		for {
			select {
			case <- ticker.C:
				queue.SendMessage("Corohlo!")
				time.Sleep(500)
				out<-"Check now"
			case <- quit:
				ticker.Stop()
				return
			}
		}
	}()
}

func doDatCrap() {
	var queueName string = os.Getenv("SQS_QUEUE")
	if len(queueName) == 0 {
		log.Fatal("Failed to get queue name from environment")
	}
	log.Print("Detected queue name:", queueName)

	// Setup AWS auth and client
	auth, err := aws.EnvAuth()
	if err != nil {
		log.Fatal(err)
	}

	if err != nil {
		log.Fatal(err)
	}

	s := sqs.New(auth, region)
	queue, err := s.GetQueue(queueName)
	if err != nil {
		log.Fatal("Fuck!", err)
	}

	c := make(chan string)
	go receiveMessages(queue, c)
	go sendMessages(queue, c)

	time.Sleep(20 * time.Second)
}

func slice (slicerPath string, modelPath string) {
	
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}
	absSlicerPath := filepath.Join(dir, slicerPath)

	cmdArgs := filepath.Join(dir, modelPath)
	cmdArgs = modelPath
	fmt.Println(absSlicerPath, cmdArgs)

	c := exec.Command(absSlicerPath, cmdArgs)
	c.Stdout = os.Stdout
	
	if err := c.Run(); err != nil {
		fmt.Printf("%T\n", err)
		log.Fatalf("ERR!", err)
	}

}

// Loads slicing job from SQS, fetches stl file, slices it, saves gcode to S3,
// puts info back on queue
func loadAndProcess(queue * sqs.Queue, bucket * s3.Bucket) {

	log.Printf("loadAndProcess called with queue %s and bucket %s",
		queue.Name, bucket.Name)

	params := map[string]string {
		"yes": "yes",
		"MaxNumberOfMessages": "1",
		"VisibilityTimeout": "100",
	}
	resp, err := queue.ReceiveMessageWithParameters(params)
	if err != nil { log.Fatal(err) }

	log.Printf("Num of messages received: %d", len(resp.Messages))
	if len(resp.Messages) == 0 {
		log.Printf("Returning.")
		return
	} else {
		log.Printf("Message found on slicer queue:", resp.Messages[0])
		// Pull from queue so others don't see it
		_, err = queue.DeleteMessage(&resp.Messages[0])
		if err != nil { log.Fatal(err) }
	}

	

// loadFileFromS3("file")
//	slice("bin/Slic3r/bin/slic3r", "model.stl")
}

func main() {
	// Setup env
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	auth, err := aws.EnvAuth()
	if err != nil { log.Fatal(err) }

	bucketName := os.Getenv("S3_BUCKET")
	if len(bucketName) == 0 {
		log.Fatal("S3_BUCKET env variable required but not found")
	}
	queueName := os.Getenv("SQS_QUEUE")
	if len(queueName) == 0 {
		log.Fatal("SQS_QUEUE env variable required but not found")
	}
	
	// Get s3 bucket

	bucket := s3.New(auth, region).Bucket(bucketName)

	// Get sqs queue
	queue, err := sqs.New(auth, region).GetQueue(queueName)
	if err != nil { log.Fatal(err) }
	
	loadAndProcess(queue, bucket)
}
