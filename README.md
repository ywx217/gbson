<p align="center">
<a href="https://godoc.org/github.com/ywx217/gbson"><img src="https://img.shields.io/badge/api-reference-blue.svg?style=flat-square" alt="GoDoc"></a>
</p>

GBSON is a Go package inspired by [tidwall/gjson](https://github.com/tidwall/gjson), that provides a fast and simple way
to get fields from a bson binary message.

Getting Started
===============

## Install

Install the package by simply run `go get`:

```shell
➜ go get -u github.com/ywx217/gbson
```

## Performance

Benchmarks of GBSON alongside [bson](go.mongodb.org/mongo-driver/bson) is in [gbson_test.go](./gbson_test.go).

These benchmarks were run on a MacBook Pro 15" Intel Core i7@2.20GHz, use `make bench-compare` to reproduce the results.

| name                            | description                                                 | time/op     | alloc/op    | allocs/op  |
|---------------------------------|-------------------------------------------------------------| -----       | -----       |------------|
| GetAllFields/bson_unmarshal-12  | Unmarshal into bson.D using mongo-driver/bson               |  182µs ± 4% | 67.2kB ± 0%  | 1.11k ± 0% |
| GetAllFields/gbson_get_all-12   | Gets all first level fields using gbson.Get                 | 83.7µs ± 0% |  0.00B       | 0.00       |
| GetAllFields/gbson_get_first-12 | Gets the first single key with gbson.Get                    | 33.3ns ± 3% |  0.00B       | 0.00       |
| GetAllFields/gbson_get_last-12  | Gets the last single key with gbson.Get                     | 1.68µs ± 2% |  0.00B       | 0.00       |
| GetAllFields/gbson_map-12       | Parse the document into a map[string]Result using gbson.Map | 13.2µs ± 5% | 15.9kB ± 0%  | 111 ± 0%   |


* size: 4885 Bytes
* content
  * 50x integer keys
  * 50x integer array keys, each array has 10 elements
