# CLI

# NAME

chrb - run ruby commands

# SYNOPSIS

chrb

```
[--default-ruby-version]=[value]
```

**Usage**:

```
chrb [GLOBAL OPTIONS] [command [COMMAND OPTIONS]] [ARGUMENTS...]
```

# GLOBAL OPTIONS

**--default-ruby-version**="": default ruby to use when no ruby is specified


# COMMANDS

## list

list all installed rubies

**--format**="": text|json (default: text)

## use

prints the shell commands to eval to use the ruby

## exec

execute a command with a ruby

## matrix

run a command in a matrix of rubies

**--ruby**="": the rubies to run the command on (default: [])
