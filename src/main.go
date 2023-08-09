package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/secretsmanager"

	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

type Credentials struct {
	AccessKeyID     string `json:"AccessKeyID"`
	SecretAccessKey string `json:"SecretAccessKey"`
}

func getCredentials() (*Credentials, error) {
	// Specify the region
	awsConfig := aws.Config{Region: aws.String("us-east-1")} // Change to your region

	// Start session with the specified region
	sess, err := session.NewSessionWithOptions(session.Options{
		Config: awsConfig,
	})
	if err != nil {
		log.Printf("Error creating session: %v", err)
		return nil, err
	}

	// Get secret value
	svc := secretsmanager.New(sess)
	secretValue, err := svc.GetSecretValue(
		&secretsmanager.GetSecretValueInput{SecretId: aws.String("AppCredentials")},
	)

	// Handle errors
	if err != nil {
		log.Printf("Secrets not returned: %v", err)
		return nil, err
	}

	// Unmarshal credentials
	var credentials Credentials
	secret := *secretValue.SecretString
	secretBytes := []byte(secret)
	if err := json.Unmarshal(secretBytes, &credentials); err != nil {
		log.Printf("Error unmarshaling credentials: %v", err)
		return nil, err
	}

	return &credentials, nil
}

// Create DynamoDB client
func createDBClient() *dynamodb.DynamoDB {

	creds, err := getCredentials()
	if err != nil {
		log.Fatalf("Failed to get credentials: %v", err)
	}

	// Create AWS config with credentials and region
	config := &aws.Config{
		Credentials: credentials.NewStaticCredentials(creds.AccessKeyID, creds.SecretAccessKey, ""),
		Region:      aws.String("us-east-1"), // Change to your region
	}

	// Create session with config
	sess, err := session.NewSession(config)
	if err != nil {
		log.Fatalf("Failed to create session: %v", err)
	}

	// Create and return client
	return dynamodb.New(sess)

}

var cidList []string

type Metadata struct {
	Image       string `json:"image"`
	Description string `json:"description"`
	Name        string `json:"name"`
}

func loadCIDs() {

	// Open CIDs file
	cidFile, err := os.Open("../data/ipfs_cids.csv")
	if err != nil {
		log.Fatalf("Failed to open CID file: %v", err)
	}
	defer cidFile.Close()

	// Create scanner
	scanner := bufio.NewScanner(cidFile)

	// Scan file line by line
	for scanner.Scan() {
		cid := scanner.Text()

		// Append each CID to list
		cidList = append(cidList, cid)
	}

	// Check for errors
	if err := scanner.Err(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

}

const gateway = "https://blockpartyplatform.mypinata.cloud/ipfs/%s"

func fetchMetadata(cid string) Metadata {

	url := fmt.Sprintf(gateway, cid)

	resp, err := http.Get(url)
	if err != nil {
		// Handle error
		log.Printf("Failed to fetch URL: %v", err)
		return Metadata{}
	}
	defer resp.Body.Close()

	var result Metadata

	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&result)
	if err != nil {
		// Handle decode error
		return Metadata{}
	}

	db := createDBClient()

	input := &dynamodb.PutItemInput{
		TableName: aws.String("Metadata"),
		Item: map[string]*dynamodb.AttributeValue{
			"CID":         {S: aws.String(cid)},
			"Image":       {S: aws.String(result.Image)},
			"Description": {S: aws.String(result.Description)},
			"name":        {S: aws.String(result.Name)},
		},
	}

	_, err = db.PutItem(input)

	if err != nil {
		log.Printf("Error saving to DynamoDB: %v", err)
	}

	return result
}

func handleGetMetadata(w http.ResponseWriter, r *http.Request) {

	var results []Metadata

	for _, cid := range cidList {
		meta := fetchMetadata(cid)
		results = append(results, meta)
	}

	json.NewEncoder(w).Encode(results)

}

func handleGetSingleMetadata(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	cid := vars["cid"]

	result := fetchMetadata(cid)

	json.NewEncoder(w).Encode(result)
}

func scanTokens(db *dynamodb.DynamoDB) ([]Metadata, error) {
	var metadatas []Metadata
	input := &dynamodb.ScanInput{
		TableName: aws.String("Metadata"),
	}

	// Paginate through the results
	for {
		result, err := db.Scan(input)
		if err != nil {
			return nil, err
		}

		for _, item := range result.Items {
			var metadata Metadata
			err := dynamodbattribute.UnmarshalMap(item, &metadata)
			if err != nil {
				return nil, err
			}
			metadatas = append(metadatas, metadata)
		}

		// Check for more results
		if result.LastEvaluatedKey == nil {
			break
		}
		input.ExclusiveStartKey = result.LastEvaluatedKey
	}

	return metadatas, nil
}

func handleGetAllTokens(w http.ResponseWriter, r *http.Request) {
	db := createDBClient()

	metadatas, err := scanTokens(db)
	if err != nil {
		log.Printf("Error scanning DynamoDB table: %v", err)
		http.Error(w, "Failed to retrieve tokens", http.StatusInternalServerError)
		return
	}

	// Encode the result as JSON
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(metadatas)
	if err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// In main function
func main() {
	loadCIDs()

	router := mux.NewRouter()

	router.HandleFunc("/tokens", handleGetAllTokens).Methods("GET")
	router.HandleFunc("/tokens/{cid}", handleGetSingleMetadata).Methods("GET")

	log.Println("Server starting on port 8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}
