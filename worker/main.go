
package main

import (
	"fmt"
	"log"
)

import (
	"github.com/AdRoll/goamz/s3"
	"github.com/AdRoll/goamz/aws"
	"github.com/AdRoll/goamz/sqs"
	"github.com/joho/godotenv"
)


func trySendMessage(auth aws.Auth, text string) {
	s := sqs.New(auth, aws.USEast)
	resp, err := s.ListQueues("")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("oi → ", resp.QueueUrl)
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	auth, err := aws.EnvAuth()
	if err != nil {
		log.Fatal(err)
	}
	client := s3.New(auth, aws.USEast)
	resp, err := client.GetService()

	if err != nil {
		log.Fatal(err)
	}

	log.Print(fmt.Sprintf("%T %+v", resp.Buckets[0], resp.Buckets[0]))
	fmt.Println("Oiem.")

	trySendMessage(auth, "Oi, Felipe!")
}
