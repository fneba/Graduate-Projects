package main

import (
	"io"
	"log"
	"net"
	"os"

	pb "countprimes.com/P2_primes/proto"

	"google.golang.org/grpc"
)

type fileServer struct {
	pb.UnimplementedFileServerServer

	// mu sync.Mutex
}

func (s *fileServer) GetFileSegment(req *pb.GetFileSegmentRequest, stream pb.FileServer_GetFileSegmentServer) error {
	log.Printf("Attempting to open file: %s", req.Pathname)
	file, err := os.Open(req.Pathname)
	if err != nil {
		log.Printf("Failed to open file: %v", err)
		return err
	}
	defer file.Close()
	log.Printf("File opened successfully: %s", req.Pathname)

	// seek to start of segment
	_, err = file.Seek(int64(req.Start), io.SeekStart)
	if err != nil {
		return err
	}

	buffer := make([]byte, req.ChunkSize) // buffer of required ChunkSize
	for {
		// bytesToRead := min(int(requiredBytes), len(buffer))
		n, err := file.Read(buffer)
		if n > 0 {
			if sendErr := stream.Send(&pb.GetFileSegmentResponse{Chunk: buffer[:n]}); sendErr != nil {
				return sendErr
			}
		}
		if err != nil {
			if err != io.EOF {
				break
			}
			return err
		}

	}

	return nil
}

func main() {

	lis, err := net.Listen("tcp", "localhost:50053")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterFileServerServer(grpcServer, &fileServer{})
	log.Printf("fileserver listening at %v", lis.Addr())
	grpcServer.Serve(lis)
	log.Printf("Done serving!")
}
