# Start from the latest golang base image
FROM golang:latest as builder

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY ./star/go.mod ./star/go.sum ./

# Copy the local dependency
COPY ./magnetar ../magnetar
COPY ./kuiper ../kuiper
COPY ./oort ../oort
COPY ./meridian ../meridian

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy everything from the current directory to the Working Directory inside the container
COPY ./star/ .

# Build the Go app
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd

######## Start a new stage from scratch #######
FROM alpine:latest

WORKDIR /root/

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /app/main .

# # Command to run the executable
# CMD ["./main"]

# Command to run docker daemon and run the executable
CMD ["sh", "-c", "dockerd & ./main"]