package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

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

	//allPieces := make([]Lego, 0)

	c := colly.NewCollector()
	c2 := c.Clone()

	var token string

	payload := []byte(`{"operationName":"Login","variables":{},"query":"mutation Login($forceCtLogin: Boolean, $forceCtExpiry: Boolean) {\n  login(forceCtLogin: $forceCtLogin, forceCtExpiry: $forceCtExpiry)\n}"}`)

	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting: ", r.URL.String())
		r.Headers.Set("Content-Type", "application/json;charset=UTF-8")
	})

	c2.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting: ", r.URL.String())
		//fmt.Println("Token: ", token)
		r.Headers.Set("Content-Type", "application/json;charset=UTF-8")
		r.Headers.Set("Authorization", token)
	})

	c.OnResponse(func(r *colly.Response) {
		fmt.Println("Got a response from", r.Request.URL)

		var data map[string]interface{}

		err := json.Unmarshal([]byte(r.Body), &data)
		if err != nil {
			fmt.Println(err)
		}

		key := "data"
		value, ok := data[key]
		if !ok {
			fmt.Printf("key %q not found in data\n", key)
			return
		}

		dataMap, ok := value.(map[string]interface{})
		if !ok {
			fmt.Printf("value for key %q is not a map[string]interface{}\n", key)
			return
		}

		//fmt.Println(dataMap["login"])
		token = dataMap["login"].(string)
		payload = []byte(`{"operationName":"PickABrickQuery","variables":{"input":{}},"query":"query PickABrickQuery($input: ElementQueryArgs, $sku: String) {\n  __typename\n  elements(input: $input) {\n    count\n    facets {\n      ...FacetData\n      __typename\n    }\n    sortOptions {\n      ...Sort_SortOptions\n      __typename\n    }\n    results {\n      ...ElementLeafData\n      __typename\n    }\n    set {\n      id\n      type\n      name\n      imageUrl\n      instructionsUrl\n      pieces\n      inStock\n      price {\n        formattedAmount\n        __typename\n      }\n      __typename\n    }\n    total\n    __typename\n  }\n}\n\nfragment FacetData on Facet {\n  id\n  key\n  name\n  labels {\n    count\n    key\n    name\n    children {\n      count\n      key\n      name\n      ... on FacetValue {\n        value\n        __typename\n      }\n      __typename\n    }\n    ... on FacetValue {\n      value\n      __typename\n    }\n    ... on FacetRange {\n      from\n      to\n      __typename\n    }\n    __typename\n  }\n  __typename\n}\n\nfragment Sort_SortOptions on SortOptions {\n  id\n  key\n  direction\n  label\n  analyticLabel\n  __typename\n}\n\nfragment ElementLeafData on Element {\n  id\n  name\n  categories {\n    name\n    key\n    __typename\n  }\n  inStock\n  ... on SingleVariantElement {\n    variant {\n      ...ElementLeafVariant\n      __typename\n    }\n    __typename\n  }\n  ... on MultiVariantElement {\n    variants {\n      ...ElementLeafVariant\n      __typename\n    }\n    __typename\n  }\n  __typename\n}\n\nfragment ElementLeafVariant on ElementVariant {\n  id\n  price {\n    centAmount\n    formattedAmount\n    __typename\n  }\n  attributes {\n    designNumber\n    colourId\n    deliveryChannel\n    maxOrderQuantity\n    system\n    quantityInSet(sku: $sku)\n    indexImageURL\n    __typename\n  }\n  __typename\n}"}`)
		c2.PostRaw("https://www.lego.com/api/graphql/PickABrickQuery", payload)
	})

	c2.OnResponse(func(r *colly.Response) {
		fmt.Println("Got a response from", r.Request.URL)
		//fmt.Println(string(r.Body))
		var data map[string]interface{}

		err := json.Unmarshal([]byte(r.Body), &data)
		if err != nil {
			fmt.Println(err)
		}

		writeJSON(data)
		uploadToS3()
	})

	c.OnError(func(r *colly.Response, e error) {
		fmt.Println("Got this error:", e)
	})

	c2.OnError(func(r *colly.Response, e error) {
		fmt.Println("Got this error:", e)
	})

	c.PostRaw("https://www.lego.com/api/graphql/Login", payload)
}

func writeJSON(data map[string]interface{}) {
	file, err := json.MarshalIndent(data, "", " ")
	if err != nil {
		log.Println("Unable to create json file")
		return
	}

	_ = os.WriteFile("legoparts.json", file, 0644)
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

	jsonFile, err := os.Open("legoparts.json")
	// if we os.Open returns an error then handle it
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("Successfully Opened legoparts.json")
	// defer the closing of our jsonFile so that we can parse it later on
	defer jsonFile.Close()

	byteValue, _ := io.ReadAll(jsonFile)

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
		Bucket:      aws.String("rebrick"),          // bucket's name
		Key:         aws.String("legoparts.json"),   // files destination location
		Body:        bytes.NewReader(byteValue),     // content of the file
		ContentType: aws.String("application/json"), // content type
	}

	output, err := uploader.UploadWithContext(context.Background(), input)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(output)
}
