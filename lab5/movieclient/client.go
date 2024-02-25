// Package main imlements a client for movieinfo service
package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"gitlab.com/Joelle-Bailey/CloudNativeCourse/lab5/movieapi"
	"google.golang.org/grpc"
)

const (
	address      = "localhost:50051"
	defaultTitle = "Pulp fiction"
)

func main() {
	// Set up a connection to the server.
	conn, err := grpc.Dial(address, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := movieapi.NewMovieInfoClient(conn)

	// Contact the server and print out its response.
	title := defaultTitle

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if len(os.Args) > 3 {
		title = os.Args[1]
		director := os.Args[3]

		var cast []string
		var i int = 4

		for i < len(os.Args) {
			cast = append(cast, os.Args[i])
			i++
		}

		year, _ := strconv.ParseInt(os.Args[2], 10, 32)

		s, err := c.SetMovieInfo(ctx, &movieapi.MovieData{Title: title, Year: int32(year), Director: director, Cast: cast})

		if err != nil {
			log.Fatalf("could not add movie: %v", err)
		}
		log.Printf("%s", s)

	} else {

		if len(os.Args) > 1 {
			title = os.Args[1]
		}

		r, err := c.GetMovieInfo(ctx, &movieapi.MovieRequest{Title: title})
		if err != nil {
			log.Fatalf("could not get movie info: %v", err)
		}
		log.Printf("Movie Info for %s %d %s %v", title, r.GetYear(), r.GetDirector(), r.GetCast())
	}

}
