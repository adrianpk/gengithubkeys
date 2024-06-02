# GenGitHubKeys

This tool generates an SSH key pair, adds the key to the SSH agent, and uploads the public key to GitHub.

**NOTE: This project is currently a work in progress and is not yet usable.**

## Prerequisites

- Go
- A GitHub account

## Installation

Clone the repository and navigate to the project directory:

```sh
git clone https://github.com/adrianpk/gengithubkeys.git
cd gengithubkeys
```

Build the project:

```sh
make build
```

Install the binary system-wide:

```sh
sudo make install
```

## Usage

Run the `gengithubkeys` command. You will be prompted to enter your GitHub email and a personal access token:

```sh
gengithubkeys
```

## Notes

Please note that this application is still under development.
The feature for uploading keys to GitHub is not yet implemented. 

## License

[MIT](https://choosealicense.com/licenses/mit/)

Follow the prompts to generate and upload your SSH key.

## License

[MIT](https://choosealicense.com/licenses/mit/)