# Use the official Go image as the base image
FROM golang:latest

# Set the working directory inside the container
WORKDIR /app

# Copy the Go source code into the container
COPY . .

# Build the Go application
RUN go build -o app

# Expose the port the application runs on
EXPOSE 8080

# Command to run the application when the container starts
CMD ["./app"]
