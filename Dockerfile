# Use the official Golang image as the base image
FROM golang:1.17

# Set the working directory inside the container
WORKDIR /app

# Copy the go module files first to leverage Docker cache
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code from the 'src' directory to the working directory inside the container
COPY src/ ./src

# Copy any other necessary files or directories, such as 'data'
COPY data/ ./data

# Build the Go app, specifying the path to the main.go file
RUN go build -o main ./src/main.go

# Expose port 8080 to communicate with the application
EXPOSE 8080

# Command to run the application when the docker container starts
CMD ["./main"]
