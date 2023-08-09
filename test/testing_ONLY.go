
// this command would NOT nommally be present in the repository
// it is here for testing purposes ONLY

// aws secretsmanager create-secret --name AppCredentials --description "AWS Credentials" --secret-string '{"accessKeyID": "AKIA4VS6JNRBIJMJYQND", "secretAccessKey": "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"}' --region us-east-1

// aws secretsmanager create-secret \ --name AppCredentials \ --description "AWS Credentials" \ --secret-string file://secret.json \ --region us-east-1
