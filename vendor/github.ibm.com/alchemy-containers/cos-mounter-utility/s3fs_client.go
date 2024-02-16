package main

import (
	"context"
	"fmt"
	"log"

	"google.golang.org/grpc"
	"github.ibm.com/alchemy-containers/cos-mounter-utility/s3fs"
)

func main() {
    // Parse command-line arguments to get the Unix domain socket endpoint
    socketEndpoint := flag.String("socket", "", "Unix domain socket endpoint")
    flag.Parse()

    if *socketEndpoint == "" {
        log.Fatal("Missing required flag: -socket")
    }

    // Create a Unix domain socket connection
    conn, err := grpc.DialContext(
        context.TODO(),
        *socketEndpoint,
        grpc.WithInsecure(),
        grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
            return net.DialTimeout("unix", addr, timeout)
        }),
    )
    if err != nil {
        log.Fatalf("Failed to connect to gRPC server: %v", err)
    }
    defer conn.Close()

    client := s3fs.NewS3FSServiceClient(conn)

    // Call methods on the gRPC server
    mountResponse, err := client.Mount(context.TODO(), &s3fs.MountRequest{
        Args: []string{"your_bucket", "/path/to/mount/point", "-o", "passwd_file=/dev/stdin", "-o", "url=https://your_bucket.s3.amazonaws.com"},
    })
    if err != nil {
        log.Fatalf("Mount request failed: %v", err)
    }
    fmt.Println(mountResponse.Message)

    // Perform file operations on the mounted S3FS

    // Unmount S3FS
    unmountResponse, err := client.Unmount(context.TODO(), &s3fs.UnmountRequest{
        Args: []string{"/path/to/mount/point"},
    })
    if err != nil {
        log.Fatalf("Unmount request failed: %v", err)
    }
    fmt.Println(unmountResponse.Message)
}
