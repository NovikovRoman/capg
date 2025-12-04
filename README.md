# Clean Architecture with Postgres

> This is a Go application for creating a clean architecture of the application being developed.

## Table of Contents

- [Clean Architecture with Postgres](#clean-architecture-with-postgres)
- [Table of Contents](#table-of-contents)
  - [Build](#build)
  - [Usage](#usage)
    - [Entity And Repository](#entity-and-repository)
    - [Array Structure For JSON Columns](#array-structure-for-json-columns)
    - [Structure For JSON Columns](#structure-for-json-columns)
  - [Help](#help)
  - [License](#license)

### Build

```shell script
make && sudo mv bin/capg /usr/local/bin/
```

### Usage

#### Entity And Repository

Creating a entity with repository.

```shell script
capg e name
```

or

```shell script
capg entity name [-s|--simple] or [-e|--empty]
```

- `name` - entity name (required).
- `-s`, `--simple` - simple entity (ID only).
- `-e`, `--empty` - empty entity (without fields).

if migration tools were missing, they will be created.

#### Array Structure For JSON Columns

Creating an array structure for a json column.

```shell script
capg a name
```

or

```shell script
capg array name
```

- `name` - struct name (required).

#### Structure For JSON Columns

Creating a structure for a json column.

```shell script
capg s name
```

or

```shell script
capg struct name
```

- `name` - struct name (required).

### Help

Any of the options:

```shell script
capg -h|--help
```

### License

[MIT License](LICENSE) © Roman Novikov
