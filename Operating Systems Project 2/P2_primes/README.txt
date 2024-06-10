This project is a GoLang fileserver process that allows workers to fetch 
a segment of a file in chunks via a gRPC call. The fileserver implements 
server--side streaming gRPC for providing the chunks of a segment. 
The Fileserver process is started and terminated explicitly by the user.

This is an extension of Project 1 (also in the repository) where instead of
workers as Go Routines, the workers are independent processes 
(running on possible different machines) communicating with the 
dispatcher, consolidator, and a fileserver via gRPC over TCP.

When running this project you need to have terminals for the workers
folder (may need multiple if using multiple workers), for the main
folder, and for the fileserver folder.

To start off, open three terminals, one for each folder and run the
command below:

1. go build .

This will give you the executables necessary to run the system.
After that the system is set up and you can refer to the report
pdf for specific instructions and input combinations to run the
prime counting system.
