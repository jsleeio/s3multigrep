# s3multigrep

## what is it?

`s3multigrep` is a tool for discovering and searching text objects in AWS S3
buckets, including objects compressed with `gzip` or `bzip2`. It was developed
with searching bulk log data in mind.

## how do I use it?

AWS credentials are presumed present in the environment, such as environment
variables or an IAM instance profile attached to an EC2 instance.

```
$ ./s3multigrep -help
Usage of ./s3multigrep:
  -bucket string
    	Name of S3 bucket to operate in
  -content-match string
    	String match on S3 object key
  -key-match string
    	String match on S3 object key
  -prefix string
    	Bucket object base prefix
  -region string
    	AWS region to operate in (default "us-west-2")
  -show-keys
    	Include S3 keys with matching lines, like traditional grep
```

## example usage

Typical usage might look like the below example:

```
$ ./s3multigrep -bucket=MYBUCKET -prefix=2018/08/05 -key-match=myapp-errors -content-match=[Ee]xception -show-keys
```

In this example:

* objects in the AWS S3 bucket `MYBUCKET` will be searched (`-bucket` option)
* object keys not matching the prefix `2018/08/05` will be ignored (`-prefix`
  option)
* object keys not matching the regular expression `myapp-errors` will be
  ignored (`-key-match` option)
* object content lines matching the regular expression `[Ee]xception` will be
  printed (`-content-match` option)
* the relevant object key will be included in each content match output, like
  filename display with regular `grep` (`-show-keys` option)
