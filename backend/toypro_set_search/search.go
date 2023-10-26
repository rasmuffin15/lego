package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/gocolly/colly"
)

type SecretData struct {
	RebrickKey string `json:"key"`
	RebrickID  string `json:"id"`
}

func main() {
	fmt.Println("Hello, World!")

	c := colly.NewCollector()

	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting: ", r.URL.String())
	})

	c.OnResponse(func(r *colly.Response) {
		fmt.Println("Got a response from", r.Request.URL)
		//fmt.Println(string(r.Body))
	})

	c.OnHTML("span.c_title.c_title--size-3.c_product-block__title", func(e *colly.HTMLElement) {
		title := strings.Fields(e.Text)
		partNumber := title[1]
		setNameTemp := title[2:]
		setName := strings.Join(setNameTemp, " ")

		if strings.Contains(setName, "advent calendar") {
			fmt.Println("Not an advent calendar")
		} else if strings.Contains(setName, "Advent Calendar") {
			fmt.Println("Not an Advent Calendar")
		} else if strings.Contains(setName, "Minifig") {
			fmt.Println("Not a Minifig")
		} else {
			fmt.Println("Part Number: ", partNumber)
			fmt.Println("Set Name: ", setName)
			csvRow := []string{partNumber, setName}
			file, err := os.OpenFile("setNumberName.csv", os.O_APPEND|os.O_WRONLY, os.ModeAppend)
			if err != nil {
				log.Fatalf("failed opening file: %s", err)
			}
			defer file.Close()

			partSet := csv.NewWriter(file)
			defer partSet.Flush()

			err = partSet.Write(csvRow)
			if err != nil {
				log.Fatalf("failed writing to file: %s", err)
			}
		}
	})

	headers := []string{"Set Number", "Set Name"}
	csvFile, err := os.Create("setNumberName.csv")

	if err != nil {
		log.Fatalf("failed creating file: %s", err)
	}

	writer := csv.NewWriter(csvFile)

	err = writer.Write(headers)
	if err != nil {
		log.Fatalf("failed writing to file: %s", err)
	}

	writer.Flush()
	csvFile.Close()

	for i := 1; i <= 255; i++ {
		website := fmt.Sprintf("https://www.toypro.com/us/list/complete-sets?sortby=&fage=&fprice=&page=%d", i)
		c.Visit(website)
	}
	uploadToS3()
}

func getAwsCredentials() SecretData {
	secretName := "rebrick_s3"
	region := "us-west-2"

	config, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		log.Fatal(err)
	}

	// Create Secrets Manager client
	svc := secretsmanager.NewFromConfig(config)

	input := &secretsmanager.GetSecretValueInput{
		SecretId:     aws.String(secretName),
		VersionStage: aws.String("AWSCURRENT"), // VersionStage defaults to AWSCURRENT if unspecified
	}

	result, err := svc.GetSecretValue(context.TODO(), input)
	if err != nil {
		// For a list of exceptions thrown, see
		// https://docs.aws.amazon.com/secretsmanager/latest/apireference/API_GetSecretValue.html
		log.Fatal(err.Error())
	}

	var secretString string
	if result.SecretString != nil {
		secretString = *result.SecretString
	}

	var secretData SecretData
	err = json.Unmarshal([]byte(secretString), &secretData)
	if err != nil {
		panic(err.Error())
	}

	return secretData
}

func uploadToS3() {

	awsCredentials := getAwsCredentials()

	rebrickKey := awsCredentials.RebrickKey
	rebrickID := awsCredentials.RebrickID

	csvFile, err := os.Open("setNumberName.csv")
	// if we os.Open returns an error then handle it
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("Successfully Opened setNumberName.csv")
	// defer the closing of our jsonFile so that we can parse it later on
	defer csvFile.Close()

	byteValue, _ := io.ReadAll(csvFile)

	s3Config := &aws.Config{
		Region:      aws.String("us-west-2"),
		Credentials: credentials.NewStaticCredentials(rebrickKey, rebrickID, ""),
	}
	s3Session, error := session.NewSession(s3Config)

	if error != nil {
		fmt.Println(error)
	}

	uploader := s3manager.NewUploader(s3Session)

	input := &s3manager.UploadInput{
		Bucket:      aws.String("rebrick"),           // bucket's name
		Key:         aws.String("setNumberName.csv"), // files destination location
		Body:        bytes.NewReader(byteValue),      // content of the file
		ContentType: aws.String("text/csv"),          // content type
	}

	output, err := uploader.UploadWithContext(context.Background(), input)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(output)
}
