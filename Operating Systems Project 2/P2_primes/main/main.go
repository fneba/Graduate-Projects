package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"sync"
	"time"

	pb "countprimes.com/P2_primes/proto"

	"google.golang.org/grpc"
)

type dispatcherServer struct {
	pb.UnimplementedDispatcherServer
	jobQueue chan Job
}

type consolidatorServer struct {
	pb.UnimplementedConsolidatorServer
	resultQueue chan Result
}

// Set up Job and Result for queueing
// Job with descriptor
type Job struct {
	pathname string
	start    int64
	length   int64
}

// Result of worker completing Job add the stuff from job struct AND primes count
type Result struct {
	countedPrimes int
	pathname      string
	start         int64
	length        int64
}

func (s *dispatcherServer) GetJob(ctx context.Context, req *pb.GetJobRequest) (*pb.GetJobResponse, error) {

	if len(s.jobQueue) == 0 {
		log.Printf("No jobs available, waiting for job...")
	}

	job := <-s.jobQueue // Attempt to dequeue a job
	log.Printf("Dispatching job: %+v", job)

	return &pb.GetJobResponse{
		Pathname: job.pathname,
		Start:    job.start,
		Length:   job.length,
	}, nil

}

func (s *consolidatorServer) PushPrimes(ctx context.Context, req *pb.PushPrimesRequest) (*pb.PushPrimesResponse, error) {

	log.Printf("Received %d primes for job from %d to %d", req.Primes, req.Start, req.Start+req.Length)
	s.resultQueue <- Result{
		countedPrimes: int(req.Primes),
		pathname:      req.Pathname,
		start:         req.Start,
		length:        req.Length,
	}
	return &pb.PushPrimesResponse{}, nil
}

// function for dispatcher
func dispatcher(jobQueue chan Job, filename string, N int64, jobs *int, wg *sync.WaitGroup) {

	defer close(jobQueue)
	go makeDispatcherServer(jobQueue)

	fileData, err := os.Stat(filename)
	if err != nil {
		log.Fatalf("Failed to stat file: %v", err)
	}

	file_size := fileData.Size()
	if file_size == 0 {
		log.Printf("File is empty, no jobs to queue.")
		return
	}

	// for loop to start at 0, and go up to the length of the file and break into segments of size N
	for segments_Start := int64(0); segments_Start < file_size; segments_Start += N {
		size := N
		// if it goes beyond the length of the file then shorten to the size of the segment
		if segments_Start+size > file_size {
			size = file_size - segments_Start
		}
		jobQueue <- Job{pathname: filename, start: segments_Start, length: size}
		log.Printf("Queued job from %d to %d", segments_Start, segments_Start+size)
		*jobs++
	}

	log.Printf("All jobs queued, total: %d", *jobs)

}

// function for consolidator; will sum the results
func consolidator(resultQueue chan Result, sumPrimes *int, jobs *int, wg *sync.WaitGroup) {
	defer wg.Done()

	go makeConsolidatorServer(resultQueue, jobs)

	// pulls "result" from result channel and sums until empty
	for result := range resultQueue {
		*sumPrimes += result.countedPrimes
		*jobs--
		log.Printf("Received and added %d primes. Remaining jobs: %d", result.countedPrimes, *jobs)
		if *jobs == 0 {
			log.Printf("All jobs processed. Total primes: %d", *sumPrimes)
			close(resultQueue)
		}
	}

}

// make dispatcher server
func makeDispatcherServer(jobQueue chan Job) {

	lis, err := net.Listen("tcp", "localhost:50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterDispatcherServer(grpcServer, &dispatcherServer{jobQueue: jobQueue})
	log.Printf("dispatcher listening at %v", lis.Addr())
	grpcServer.Serve(lis)
}

// make consolidator server
func makeConsolidatorServer(resultQueue chan Result, jobs *int) {

	lis, err := net.Listen("tcp", "localhost:50052")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterConsolidatorServer(grpcServer, &consolidatorServer{resultQueue: resultQueue})
	log.Printf("consolidator listening at %v", lis.Addr())
	grpcServer.Serve(lis)

	if *jobs == 0 {
		grpcServer.GracefulStop()
	}
}

func main() {

	// Take in command line args
	filename_ptr := flag.String("filename", "/Users/fneba/Desktop/P2_primes/numbers.dat", "pathname to the input file")
	N_ptr := flag.Int64("N", 1*1024, "size of file segments (bytes)")
	// C_point := flag.Int64("C", 1*1024, "chunk size for reading file segments (bytes)")
	flag.Parse()

	// Record the start time
	start := time.Now()

	// create job and result channels and other important constants
	var wg sync.WaitGroup
	jobQueue := make(chan Job)
	resultQueue := make(chan Result)
	jobs := 0
	sumPrimes := 0

	// dispatcher goroutine
	go dispatcher(jobQueue, *filename_ptr, *N_ptr, &jobs, &wg)

	// consolidator goroutine
	wg.Add(1)
	go consolidator(resultQueue, &sumPrimes, &jobs, &wg)

	// Halt consolidator until consolidator is finished
	wg.Wait()

	// Print program runtime
	runtime := time.Since(start)
	log.Printf("elapsed time: %s", runtime)

}
