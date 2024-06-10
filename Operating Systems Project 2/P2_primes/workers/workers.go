package main

import (
	"context"
	"encoding/binary"
	"flag"
	"io"
	"log"
	"math/big"
	"strings"
	"time"

	pb "countprimes.com/P2_primes/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// this is the main functionality of the workers in counting primes, no longer need the buffer since
// the we don't have to read it from the file and the file server provides stream of chunks
func workerProcess(segment []byte) int {
	var primes int = 0
	segmentLength := int64(len(segment))

	for i := int64(0); i < segmentLength; i += 8 {
		if i+8 > segmentLength {
			break
		}
		num := binary.LittleEndian.Uint64(segment[i : i+8])
		z := new(big.Int).SetUint64(num)

		if z.ProbablyPrime(20) {
			primes++
		}

	}
	return primes

}

// This function will take the stream from the file server and process it
// into bytes for me to use for workerProcess & compute primes
func processSegment(client pb.FileServerClient, req *pb.GetFileSegmentRequest) ([]byte, error) {
	log.Printf("Getting stream for current job")
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	stream, err := client.GetFileSegment(ctx, req)
	if err != nil {
		log.Printf("client.GetFileSegment failed: %v", err)
		return nil, err
	}

	var data []byte

	for {
		segment, err := stream.Recv()
		if err != nil {
			errMsg := err.Error()
			eof := strings.Contains(errMsg, "EOF")
			if (err == io.EOF) || (eof == true) {
				log.Println("End of file segment stream reached.")
				break // Proper handling of EOF, break the loop, not an error
			}
			log.Printf("Error while receiving segment: %v", err)
			return nil, err // Return errors that are not EOF
		}
		data = append(data, segment.Chunk...)
		log.Printf("Segment successfully retrieved, length: %d", len(segment.Chunk))
	}

	log.Printf("All segments received, total length: %d", len(data))
	return data, nil
}

func main() {

	// Read in command line arguments
	C_ptr := flag.Int64("C", 64, "chunk size for reading file segments (bytes)")
	flag.Parse()

	// form connections to dispatcher, consolidator and
	dConn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		log.Fatalf("could not connect to file server: %v", err)
	}
	defer dConn.Close()
	log.Println("Connected to dispat successfully")
	dClient := pb.NewDispatcherClient(dConn) // dispatcher client

	cConn, err := grpc.Dial("localhost:50052", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		log.Fatalf("Failed to connect to consolidator: %v", err)
	}
	defer cConn.Close()
	log.Println("Connected to consol successfully")
	cClient := pb.NewConsolidatorClient(cConn) // consolidator client

	fsConn, err := grpc.Dial("localhost:50053", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		log.Fatalf("could not connect to file server: %v", err)
	}
	defer fsConn.Close()
	log.Println("Connected to FS successfully")
	fsClient := pb.NewFileServerClient(fsConn) // fileserver client

	// process jobs in a loop

	for {
		// Get job from dispatcher
		jobReq := &pb.GetJobRequest{}
		job, err := dClient.GetJob(context.Background(), jobReq)
		if err != nil {
			log.Printf("Failed to get job from dispatcher: %v", err)
			continue
		}
		log.Printf("Received job: %+v", job)

		// request file segment from file server
		segmentReq := &pb.GetFileSegmentRequest{
			Pathname:  job.Pathname,
			Start:     job.Start,
			End:       job.Start + job.Length,
			ChunkSize: *C_ptr,
		}

		// retrieve stream of bytes from file server & process it
		segmentData, err := processSegment(fsClient, segmentReq)
		if err != nil {
			// log.Printf("Full segment here: %d", segmentData)
			log.Fatalf("Failed to get segment from FS: %v", err)
		}
		log.Printf("Received full segment!")

		// count primes
		primeAmount := workerProcess(segmentData)
		log.Printf("Full number of primes: %d", primeAmount)

		// send result to the consolidator
		primesReq := &pb.PushPrimesRequest{
			Primes:   int64(primeAmount),
			Pathname: job.Pathname,
			Start:    job.Start,
			Length:   job.Length,
		}

		_, err = cClient.PushPrimes(context.Background(), primesReq)
		if err != nil {
			log.Printf("Failed to send primes to consolidator: %v", err)

		} else {
			log.Printf("Successfully sent %d primes to consolidator for segment starting at %d", primeAmount, job.Start)
		}

	}

}
