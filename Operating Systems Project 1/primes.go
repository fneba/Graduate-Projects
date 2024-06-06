package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"math/rand"
	"os"
	"sync"
	"time"
)

// Set up Job and Result for queueing
// Job with descriptor
type Job struct {
	pathname string
	start    int64
	length   int64
}

// Result of worker completing Job
// add the stuff from job struct AND primes count
type Result struct {
	countedPrimes int
	pathname      string
	start         int64
	length        int64
}

// function for dispatcher
// dispatcher and consolidator need to work in tandem so there isn't a deadlock with the consolidator
// orrrr, I could just make the Consolidator wait 3 seconds
func dispatcher(jobQueue chan<- Job, filename string, N int64) {
	fileData, err := os.Stat(filename)
	if err != nil {
		panic(err)
	}

	file_size := fileData.Size()

	// for loop to start at 0, and go up to the length of the file and break
	// it into segments of size N
	for segments_Start := int64(0); segments_Start < file_size; segments_Start += N {
		size := N

		// if it goes beyond the length of the file then shorten to the size of the segment
		if segments_Start+size > file_size {
			size = file_size - segments_Start
		}
		jobQueue <- Job{pathname: filename, start: segments_Start, length: size}
	}

	close(jobQueue)

}

// Worker function to compute primes
func worker(id int, jobQueue <-chan Job, resultQueue chan<- Result, C int64, wg *sync.WaitGroup) {

	// call sleep function
	time.Sleep(time.Millisecond * time.Duration(rand.Intn(201)+400))

	// will decrement waitgroup when worker terminates
	defer wg.Done()
	// slog.Info("worker", "id", id)
	for {
		job, ok := <-jobQueue
		if !ok {
			break
		}
		primeAmount := workerProcess(job, C)
		// slog.Info("worker", "id", id, "primes", primeAmount, "Job", job)

		result := Result{countedPrimes: primeAmount, pathname: job.pathname, start: job.start, length: job.length}
		slog.Info("worker", "id", id, "result", result, "Job", job)
		resultQueue <- result
	}
	// slog.Info("worker", "id", id)
}

// this is the main functionality of the workers put in a function for testing
func workerProcess(job Job, C int64) int {
	var primes int = 0

	file, err := os.Open(job.pathname)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// Set the starting point in the file to start designated in job
	_, err = file.Seek(job.start, io.SeekStart)
	if err != nil {
		panic(err)
	}

	// Buffer to read 64-bit unassigned integers
	buf := make([]byte, C)

	// var bytesRead int
	var bytesGot int64 = 0
	// buffer size is C, so just increment each time until end of job

	for bytesGot < job.length {
		// bytes read into buffer plus the error; then erroe handling
		bytesRead, err := file.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			panic(err)
		}

		// this is to loop thru the chunk 8 bytes at a time
		// slog.Info("worker", "bytesRead", bytesRead, "bytesGot", bytesGot)
		sz := min(job.length-bytesGot, int64(bytesRead))
		bytesGot += int64(bytesRead)
		for i := int64(0); i < sz; i += 8 {

			// this must convert 8 bytes to a number and check if prime
			s := buf[i : i+8]
			num := binary.LittleEndian.Uint64(s)
			z := new(big.Int).SetUint64(num)
			if z.ProbablyPrime(20) {
				primes++
			}
		}
	}
	return primes

}

// function for consolidator; will sum the results
func consolidator(resultQueue <-chan Result) {
	sumPrimes := 0

	// pulls "result" from result channel and sums until empty
	for result := range resultQueue {
		sumPrimes += result.countedPrimes

	}
	fmt.Println("Total number of Primes:", sumPrimes)

}

func main() {

	// this is for synchronicity i.e. the consolidator waiting until the workers finish before printing result
	var wg sync.WaitGroup

	// Record the start time
	startTime := time.Now()

	// Take in command line args
	filename_point := flag.String("pathname", "numbers.dat", "pathname to the input file")
	M_point := flag.Int("M", 25, "Num of workers") // was 10
	N_point := flag.Int64("N", 65536, "size of file segments (bytes)")
	C_point := flag.Int64("C", 1024, "chunk size for reading file segments (bytes)")
	flag.Parse()

	// True value of primes: 3110 for funibaNumbers.dat

	// create job and result channels
	jobQueue := make(chan Job)
	resultQueue := make(chan Result)

	// Calls for the dispatcher with jobQueue, filename and N values by ref
	go dispatcher(jobQueue, *filename_point, *N_point)

	// spawns the M workers with w as id, and other inputs to use for necessary calculations
	for w := 0; w <= *M_point; w++ {
		wg.Add(1)
		go worker(w, jobQueue, resultQueue, *C_point, &wg)

	}

	// This goroutine waits until all the workers are finished before it closes the send part of the channel which signals
	// to the consolidatorthat there are no more results to add to sumPrimes. Unfortunately, I couldn't get the comm channel
	// between the dispatcher and consolidator to work. So for functionality (and grade) purposes, I did the manual way.

	go func() {
		wg.Wait()
		close(resultQueue)
	}()

	// spawn consolidator thread
	go consolidator(resultQueue)

	// Halt consolidator until workers finish
	wg.Wait()

	// Print program runtime
	runtime := time.Since(startTime)
	fmt.Println("Program runtime:", runtime)
	slog.Info("message", "All workers completed", *M_point)

}
