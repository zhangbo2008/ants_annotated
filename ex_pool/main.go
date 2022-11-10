package main

import "sync"

type Person struct {
    Name string
}

//Initialing pool
var personalPool = sync.Pool{
    // New optionally specifies a function to generate
    // a value when Get would otherwise return nil.
    New: func() interface{} {
        return &Person{}
    },
}

func main() {
    // Get hold of an instance
    newPerson := personalPool.Get().(*Person)
    // Defer release function
    // After that the same instance is
    // reusable by another routine
    defer personalPool.Put(newPerson)

    // using instance
    newPerson.Name = "Jack"
}