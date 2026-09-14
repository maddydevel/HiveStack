// Copyright (c) 2026 Maddy AI Consultancy. All rights reserved.
// Use of this source code is governed by the LICENSE file.

package main

import (
    "fmt"
    "os"

    "github.com/maddydevel/HiveStack/internal/cli"
)

func main() {
    if err := cli.Run(os.Args[1:]); err != nil {
        fmt.Fprintf(os.Stderr, "hive: %v\n", err)
        os.Exit(1)
    }
}
