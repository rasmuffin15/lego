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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/gocolly/colly"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type SecretData struct {
	RebrickKey string `json:"key"`
	RebrickID  string `json:"id"`
}

func main() {
	var numberName [][]string
	var priceQuantity [][]string
	var visitLoopCondition bool
	numVisits := 0

	c := colly.NewCollector()

	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting: ", r.URL.String())
		if numVisits > 9 {
			visitLoopCondition = false
			r.Abort()
		} else {
			numVisits++
		}
	})

	c.OnResponse(func(r *colly.Response) {
		fmt.Println("Got a response from", r.Request.URL)
	})

	c.OnHTML("span.c_title.c_title--size-3.c_product-block__title", func(e *colly.HTMLElement) {
		title := strings.Fields(e.Text)
		partNumber := title[1]
		setNameTemp := title[2:]
		setName := strings.Join(setNameTemp, " ")

		//fmt.Println("Found tag span")
		//fmt.Println(partNumber)
		//fmt.Println(setNameTemp)
		//fmt.Println(setName)

		numberName = append(numberName, []string{partNumber, setName})
	})

	c.OnHTML("div.col-auto.c_product-block__pricecol", func(e *colly.HTMLElement) {
		title := strings.Fields(e.Text)
		price := title[0]
		quantity := strings.Join(title[1:], " ")

		numDollarSigns := strings.Count(quantity, "$")
		if numDollarSigns > 0 {
			quantity = title[2]
		} else {
			quantity = title[1]
		}

		//fmt.Println("Found tag div")
		//fmt.Println(price)
		//fmt.Println(quantity)

		priceQuantity = append(priceQuantity, []string{price, quantity})
	})

	headers := []string{"Part Number", "Part Name", "Price", "Quantity"}
	csvFile, err := os.Create("partNumberName.csv")

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

	visitLoopCondition = true
	i := 1
	for visitLoopCondition {
		fmt.Println("Visiting: ", i)
		website := fmt.Sprintf("https://www.toypro.com/us/set/showparts?itemid=75095&sortby=itemid&page=%d", i)
		c.Visit(website)
		i++
	}

	writeDataToCSV(numberName, priceQuantity)
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

func writeDataToCSV(numberName [][]string, priceQuantity [][]string) {

	var csvRow [][]string

	for i := 0; i < len(numberName); i++ {
		csvRow = append(csvRow, []string{numberName[i][0], numberName[i][1], priceQuantity[i][0], priceQuantity[i][1]})
	}

	file, err := os.OpenFile("partNumberName.csv", os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	if err != nil {
		log.Fatalf("failed opening file: %s", err)
	}
	defer file.Close()

	partSet := csv.NewWriter(file)
	defer partSet.Flush()

	for _, value := range csvRow {
		err = partSet.Write(value)
		if err != nil {
			log.Fatalf("failed writing to file: %s", err)
		}
	}
}

func uploadToS3() {

	awsCredentials := getAwsCredentials()

	rebrickKey := awsCredentials.RebrickKey
	rebrickID := awsCredentials.RebrickID

	csvFile, err := os.Open("partNumberName.csv")
	// if we os.Open returns an error then handle it
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("Successfully Opened partNumberName.csv")
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
		Bucket:      aws.String("rebrick"),            // bucket's name
		Key:         aws.String("partNumberName.csv"), // files destination location
		Body:        bytes.NewReader(byteValue),       // content of the file
		ContentType: aws.String("text/csv"),           // content type
	}

	output, err := uploader.UploadWithContext(context.Background(), input)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(output)
}
