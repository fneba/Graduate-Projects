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

type Job struct {
	pathname string
	start    int64
	length   int64
}

type Result struct {
	countedPrimes int
	pathname      string
	start         int64
	length        int64
}


func dispatcher(jobQueue chan<- Job, filename string, N int64) {
	fileData, err := os.Stat(filename)
	if err != nil {
		panic(err)
	}

	file_size := fileData.Size()

	//break file into segments of size N
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


func worker(id int, jobQueue <-chan Job, resultQueue chan<- Result, C int64, wg *sync.WaitGroup) {

	
	time.Sleep(time.Millisecond * time.Duration(rand.Intn(201)+400))

	
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

		
		// slog.Info("worker", "bytesRead", bytesRead, "bytesGot", bytesGot)
		sz := min(job.length-bytesGot, int64(bytesRead))
		bytesGot += int64(bytesRead)
		for i := int64(0); i < sz; i += 8 {

			// convert 8 bytes to a number and check if prime
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


func consolidator(resultQueue <-chan Result) {
	sumPrimes := 0

	
	for result := range resultQueue {
		sumPrimes += result.countedPrimes

	}
	fmt.Println("Total number of Primes:", sumPrimes)

}

func main() {

	// consolidator waits until the workers finish before printing result
	var wg sync.WaitGroup

	
	startTime := time.Now()

	
	filename_point := flag.String("pathname", "numbers.dat", "pathname to the input file")
	M_point := flag.Int("M", 25, "Num of workers") // was 10
	N_point := flag.Int64("N", 65536, "size of file segments (bytes)")
	C_point := flag.Int64("C", 1024, "chunk size for reading file segments (bytes)")
	flag.Parse()

	// True value of primes: 3110 for funibaNumbers.dat

	
	jobQueue := make(chan Job)
	resultQueue := make(chan Result)

	go dispatcher(jobQueue, *filename_point, *N_point)

	for w := 0; w <= *M_point; w++ {
		wg.Add(1)
		go worker(w, jobQueue, resultQueue, *C_point, &wg)

	}

	// This goroutine waits until all the workers are finished before it closes the send part of the channel which signals
	// to the consolidator that there are no more results to add to sumPrimes. Unfortunately, I couldn't get the comm channel
	// between the dispatcher and consolidator to work. So for functionality (and grade) purposes, I did the manual way.

	go func() {
		wg.Wait()
		close(resultQueue)
	}()

	
	go consolidator(resultQueue)

	
	wg.Wait()

	
	runtime := time.Since(startTime)
	fmt.Println("Program runtime:", runtime)
	slog.Info("message", "All workers completed", *M_point)

}
