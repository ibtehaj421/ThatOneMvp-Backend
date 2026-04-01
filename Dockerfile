# Build Stage
FROM golang:1.21-alpine AS builder

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./
COPY . .
# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod tidy
RUN go mod download

# Copy the source from the current directory to the Working Directory inside the container


# Build the Go app. 
# CGO_ENABLED=0 ensures a static binary, which works perfectly in the scratch/alpine runner
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Run Stage
FROM alpine:latest  

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /app/main .

# Expose port 8080 to the outside world
EXPOSE 8080

# Command to run the executable
CMD ["./main"]