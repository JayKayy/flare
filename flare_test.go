package main

import (
	"os"
	"testing"
)

func TestLocalAuth(t *testing.T) {
	// Will pass if you have a valid ~/kube/config file
	homeDir := os.Getenv("HOME")
	file := homeDir + "/.kube/config"
	// use local file?

	_, err := auth(&file)
	// Expected err == nil
	// Got
	if err != nil {
		t.Errorf("Error building the ~/.kube/config clientset " + err.Error())
	}

}

func TestAuthPartial(t *testing.T) {
	badFile := "test/partial_config"
	// This file is good enough to parse

	_, err := auth(&badFile)
	//fmt.Println(cs)
	//	fmt.Println(err)
	// Expected non-nil
	if err != nil {
		t.Errorf("Expected an Error but err == nil")
	}
}

func TestAuthMissing(t *testing.T) {
	missingFile := "test/324567890hfusagdf7tqwyhrnuh"
	// use local file?

	_, err := auth(&missingFile)
	// Expected non-nil
	if err == nil {
		t.Errorf("Expected an Error but err was nil")
	}
}
func TestAuthEmptyFile(t *testing.T) {
	missingFile := "test/empty_config"
	// use local file?

	_, err := auth(&missingFile)
	// Expected non-nil
	if err == nil {
		t.Errorf("Expected an Error but err was nil")
	}
}
func TestAuthNoPath(t *testing.T) {
	badFile := ""
	// use local file?

	_, err := auth(&badFile)
	// Expected non-nil
	if err == nil {
		t.Errorf("Expected an Error but err was nil")
	}
}
