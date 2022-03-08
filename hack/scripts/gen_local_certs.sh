#!/usr/bin/env bash

# Tool to create certificates for running liquid metal locally

set -o pipefail

# general vars
DEFAULT_CA_CN="Liquid Metal Dev CA"
DEFAULT_HOST_NAME="$(hostname)"
DEFAULT_CLIENT_CN_NAME="Liquid Metal Client 1"

## HELPER FUNCS
#
#
# Send a green message to stdout, followed by a new line
say() {
	[ -t 1 ] && [ -n "$TERM" ] &&
		echo "$(tput setaf 2)[$MY_NAME]$(tput sgr0) $*" ||
		echo "[$MY_NAME] $*"
}

# Send a green message to stdout, without a trailing new line
say_noln() {
	[ -t 1 ] && [ -n "$TERM" ] &&
		echo -n "$(tput setaf 2)[$MY_NAME]$(tput sgr0) $*" ||
		echo "[$MY_NAME] $*"
}

# Send a red message to stdout, followed by a new line
say_err() {
	[ -t 2 ] && [ -n "$TERM" ] &&
		echo -e "$(tput setaf 1)[$MY_NAME] $*$(tput sgr0)" 1>&2 ||
		echo -e "[$MY_NAME] $*" 1>&2
}

# Send a yellow message to stdout, followed by a new line
say_warn() {
	[ -t 1 ] && [ -n "$TERM" ] &&
		echo "$(tput setaf 3)[$MY_NAME] $*$(tput sgr0)" ||
		echo "[$MY_NAME] $*"
}

# Send a yellow message to stdout, without a trailing new line
say_warn_noln() {
	[ -t 1 ] && [ -n "$TERM" ] &&
		echo -n "$(tput setaf 3)[$MY_NAME] $*$(tput sgr0)" ||
		echo "[$MY_NAME] $*"
}

# Exit with an error message and (optional) code
# Usage: die [-c <error code>] <error message>
die() {
	code=1
	[[ "$1" = "-c" ]] && {
		code="$2"
		shift 2
	}
	say_err "$@"
	exit "$code"
}

# Exit with an error message if the last exit code is not 0
ok_or_die() {
	code=$?
	[[ $code -eq 0 ]] || die -c $code "$@"
}

create_outdir() {
    local outdir="$1"

    mkdir -p "$outdir" || die "Failed to make output directory $outdir"
}

write_cfssl_config() {
    local configpath="$1"

    cat << "EOF" > "$configpath"
{
  "signing": {
    "default": {
      "expiry": "8760h"
    },
    "profiles": {
      "intermediate": {
        "usages": ["cert sign", "crl sign"],
        "expiry": "70080h",
        "ca_constraint": {
          "is_ca": true,
          "max_path_len": 1
        }
      },
      "host": {
        "usages": [
          "signing",
          "digital signing",
          "key encipherment",
          "server auth"
        ],
        "expiry": "8760h"
      },
      "client": {
        "usages": [
          "signing",
          "digital signature",
          "key encipherment", 
          "client auth"
        ],
        "expiry": "8760h"
      }
    }
  }
}
EOF
}

# Create the certificates for a client (i.e. capmvm)
create_client_cert() {
	local cn="$1"
	local cfssl="$2"
	local cfssljson="$3"
    local outdir="$4"
    local cfgpath="$outdir/config.json"
    local ca="$outdir/intermediate-ca.pem"
    local cakey="$outdir/intermediate-ca-key.pem"

    local clientname=$(echo $cn | tr -d ' ' | tr '[:upper:]' '[:lower:]')

    clientcsr="$outdir/client-$clientname-csr.json"
    say "Creating client certficate request: $clientcsr"
    
    cat <<EOF >"$clientcsr"
{
  "CN": "$cn",
  "hosts": [""],
  "names": [
    {
      "C": "UK",
      "L": "London",
      "O": "Liquid Metal Internal",
      "OU": "Liquid Metal Internal Clients"
    }
  ]
}
EOF

    clientcert="$outdir/$clientname"
    say "Generating client certificate: $clientcert"
    $cfssl gencert \
        -ca $ca \
        -ca-key $cakey \
        -config $cfgpath \
        -profile client $clientcsr \
        | $cfssljson -bare $clientcert

	say "Client certificate created: $cn"
}

# Create the certificates for a host
create_host_cert() {
    local host="$1"
	local cn="$2"
	local cfssl="$3"
	local cfssljson="$4"
    local outdir="$5"
    local cfgpath="$outdir/config.json"
    local ca="$outdir/intermediate-ca.pem"
    local cakey="$outdir/intermediate-ca-key.pem "

    hostcsr="$outdir/$host-csr.json"
    say "Creating host certficate request: $hostcsr"
    
    cat <<EOF >"$hostcsr"
{
  "CN": "$cn",
  "hosts": ["$host"],
  "names": [
    {
      "C": "UK",
      "L": "London",
      "O": "Liquid Metal Internal",
      "OU": "Liquid Metal Internal Hosts"
    }
  ]
}
EOF

    hostcert="$outdir/$host"
    say "Generating host certificate: $hostcert"
    $cfssl gencert \
        -ca $ca \
        -ca-key $cakey \
        -config $cfgpath \
        -profile host $hostcsr \
        | $cfssljson -bare $hostcert

	say "Host certificate created: $cn"
}

# Create the certificate authority certs.
create_ca() {
	local cn="$1"
	local cfssl="$2"
	local cfssljson="$3"
    local outdir="$4"

    rootcsr="$outdir/root-csr.json"
    say "Creating root CA certficate request: $rootcsr"
    
    cat <<EOF >"$rootcsr"
{
  "CN": "$cn",
  "key": {
    "algo": "ecdsa",
    "size": 256
  },
  "names": [
    {
      "C": "UK",
      "L": "London",
      "O": "Liquid Metal Internal"
    }
  ],
  "ca": {
    "expiry": "87600h"
  }
}
EOF

    rootca="$outdir/root-ca"
    say "Generating root CA certficate: $cn"
    $cfssl gencert -initca $rootcsr \
        | $cfssljson -bare $rootca

	say "CA certificates created: $cn"
}

# Create the intermediate certificate authority certs.
create_intca() {
	local cn="$1"
	local cfssl="$2"
	local cfssljson="$3"
    local outdir="$4"
    local cfg="$5"

    intcsr="$outdir/intermediate-csr.json"
    say "Creating intermediate CA certficate request: $intcsr"
    
    cat <<EOF >"$intcsr"
{
  "CN": "$cn",
  "key": {
    "algo": "ecdsa",
    "size": 256
  },
  "names": [
    {
      "C": "UK",
      "L": "London",
      "O": "Liquid Metal Internal",
      "OU": "Liquid Metal Internal Intermediate CA"
    }
  ]
}
EOF

    intca="$outdir/intermediate-ca"
    say "Generating intermediate CA certficate: $cn"
    $cfssl genkey $intcsr \
        | $cfssljson -bare $intca

    say "Signing intermediate CA certificate"
    rootca="$outdir/root-ca.pem"
    rootcakey="$outdir/root-ca-key.pem"
    intreq="$outdir/intermediate-ca.csr"

    $cfssl sign -ca $rootca \
        -ca-key $rootcakey \
        -config $cfg \
        -profile intermediate $intreq \
        | $cfssljson -bare $intca

	say "Intermediate CA certificates created: $cn"
}

do_all_ca() {
    local cn="$1"
	local cfssl="$2"
	local cfssljson="$3"
    local outdir="$4"
    local cfgpath="$outdir/config.json"
    local intcn="$1 Intermediate"

    say "Creating output directory $outdir"
    create_outdir "$outdir"
    
    write_cfssl_config "$cfgpath"
    create_ca "$cn" "$cfssl" "$cfssljson" "$outdir"
    create_intca "$intcn" "$cfssl" "$cfssljson" "$outdir" "$cfgpath"
}

## COMMANDS
#
#
cmd_all() {
	local host="$DEFAULT_HOST_NAME"
	local cacn="$DEFAULT_CA_CN"
	local hostcn="$DEFAULT_HOST_NAME"
	local clientcn="$DEFAULT_CLIENT_CN_NAME"
	local cfssl=""
    local cfssljson=""
    local outdir=""

	while [ $# -gt 0 ]; do
		case "$1" in
		"-h" | "--help")
			cmd_ca_help
			exit 1
			;;
		"--name-ca")
			shift
			cacn="$1"
			;;
		"--name-host")
			shift
			hostcn="$1"
			;;
		"--name-client")
			shift
			clientcn="$1"
			;;
		"--host")
			shift
			host="$1"
			;;
		"--cfssl")
			shift
			cfssl="$1"
			;;
		"--cfssljson")
			shift
			cfssljson="$1"
			;;
		"--output")
			shift
			outdir="$1"
			;;
		*)
			die "Unknown argument: $1. Please use --help for help."
			;;
		esac
		shift
	done

    if [[ "$cacn" == "" ]]; then
		die "You must supply a CA common name"
	fi

    if [[ "$hostcn" == "" ]]; then
		die "You must supply a Host common name"
	fi

    if [[ "$clientcn" == "" ]]; then
		die "You must supply a client common name"
	fi

    if [[ "$cfssl" == "" ]]; then
		die "You must supply the path to the cfssl binary"
	fi

    if [[ "$cfssljson" == "" ]]; then
		die "You must supply the path to the cfssljson binary"
	fi

    if [[ "$outdir" == "" ]]; then
		die "You must supply the path to the ouput folder"
	fi

	do_all_ca "$cacn" "$cfssl" "$cfssljson" "$outdir"
	create_host_cert "$host" "$hostcn" "$cfssl" "$cfssljson" "$outdir"
	create_client_cert "$clientcn" "$cfssl" "$cfssljson" "$outdir"
}

cmd_ca() {
	local cn="$DEFAULT_CA_CN"
	local cfssl=""
    local cfssljson=""
    local outdir=""

	while [ $# -gt 0 ]; do
		case "$1" in
		"-h" | "--help")
			cmd_ca_help
			exit 1
			;;
		"-n" | "--name")
			shift
			cn="$1"
			;;
		"--cfssl")
			shift
			cfssl="$1"
			;;
		"--cfssljson")
			shift
			cfssljson="$1"
			;;
		"--output")
			shift
			outdir="$1"
			;;
		*)
			die "Unknown argument: $1. Please use --help for help."
			;;
		esac
		shift
	done

    if [[ "$cn" == "" ]]; then
		die "You must supply a name"
	fi

    if [[ "$cfssl" == "" ]]; then
		die "You must supply the path to the cfssl binary"
	fi

    if [[ "$cfssljson" == "" ]]; then
		die "You must supply the path to the cfssljson binary"
	fi

    if [[ "$outdir" == "" ]]; then
		die "You must supply the path to the ouput folder"
	fi

	do_all_ca "$cn" "$cfssl" "$cfssljson" "$outdir"
}

cmd_host() {
    local host="$DEFAULT_HOST_NAME"
	local cn="$host"
	local cfssl=""
    local cfssljson=""
    local outdir=""

	while [ $# -gt 0 ]; do
		case "$1" in
		"-h" | "--help")
			cmd_ca_help
			exit 1
			;;
		"-n" | "--name")
			shift
			cn="$1"
			;;
		"--cfssl")
			shift
			cfssl="$1"
			;;
		"--cfssljson")
			shift
			cfssljson="$1"
			;;
		"--output")
			shift
			outdir="$1"
			;;
		"--host")
			shift
			host="$1"
			;;
		*)
			die "Unknown argument: $1. Please use --help for help."
			;;
		esac
		shift
	done

    if [[ "$host" == "" ]]; then
		die "You must supply a host name"
	fi

    if [[ "$cn" == "" ]]; then
		die "You must supply a common name"
	fi

    if [[ "$cfssl" == "" ]]; then
		die "You must supply the path to the cfssl binary"
	fi

    if [[ "$cfssljson" == "" ]]; then
		die "You must supply the path to the cfssljson binary"
	fi

    if [[ "$outdir" == "" ]]; then
		die "You must supply the path to the ouput folder"
	fi

	create_host_cert "$host" "$cn" "$cfssl" "$cfssljson" "$outdir"
}

cmd_client() {
	local cn="$DEFAULT_CLIENT_CN_NAME"
	local cfssl=""
    local cfssljson=""
    local outdir=""

	while [ $# -gt 0 ]; do
		case "$1" in
		"-h" | "--help")
			cmd_ca_help
			exit 1
			;;
		"-n" | "--name")
			shift
			cn="$1"
			;;
		"--cfssl")
			shift
			cfssl="$1"
			;;
		"--cfssljson")
			shift
			cfssljson="$1"
			;;
		"--output")
			shift
			outdir="$1"
			;;
		*)
			die "Unknown argument: $1. Please use --help for help."
			;;
		esac
		shift
	done

    if [[ "$cn" == "" ]]; then
		die "You must supply a common name"
	fi

    if [[ "$cfssl" == "" ]]; then
		die "You must supply the path to the cfssl binary"
	fi

    if [[ "$cfssljson" == "" ]]; then
		die "You must supply the path to the cfssljson binary"
	fi

    if [[ "$outdir" == "" ]]; then
		die "You must supply the path to the ouput folder"
	fi

	create_client_cert "$cn" "$cfssl" "$cfssljson" "$outdir"
}

## COMMAND HELP FUNCS
#
#

cmd_all_help() {
	cat <<EOF
  all                    Complete setup for development certificates. 
    OPTIONS:
      -y                 Autoapprove all prompts (danger)
      --cfssl            The path to the cfssl binary.
      --cfssljson        The path to the cfssljson binary.
      --output           The output folder for the commands.
      --name-ca          Common name of the CA (default: Liquid Metal Dev CA)
      --name-host        Common name of the host cert (default: `($ hostname)`)
	  --name-client      Common name of the client cert (default: Liquid Metal Client 1)
      --host             Host name for the certificate (default: value returned by `$ hostname`)

EOF
}

cmd_ca_help() {
	cat <<EOF
  ca              Create the certificate authrity certs
    OPTIONS:
      --name, -n      Common name of the CA (default: Liquid Metal Dev CA)

EOF
}

cmd_host_help() {
	cat <<EOF
  ca              Create flintlock host certificates
    OPTIONS:
      --name, -n      Common name of the host cert (default: `($ hostname)`)
      --host          Host name for the certificate (default: value returned by `$ hostname`)

EOF
}

cmd_client_help() {
	cat <<EOF
  ca              Create a client certificate
    OPTIONS:
      --name, -n      Common name of the client cert (default: Liquid Metal Client 1)

EOF
}

cmd_help() {
	cat <<EOF
usage: $0 <COMMAND> <OPTIONS>

Script to create certificates for running liquid metal in dev

COMMANDS:

EOF

	cmd_all_help
	cmd_ca_help
    cmd_host_help
    cmd_client_help
}


## LET'S DO THIS THING
#
#
main() {
	if [ $# = 0 ]; then
		die "No command provided. Please use \`$0 help\` for help."
	fi

	# Parse main command line args.
	#
	while [ $# -gt 0 ]; do
		case "$1" in
		-h | --help)
			cmd_help
			exit 1
			;;
		-*)
			die "Unknown arg: $1. Please use \`$0 help\` for help."
			;;
		*)
			break
			;;
		esac
		shift
	done

	# $1 is now a command name. Check if it is a valid command and, if so,
	# run it.
	#
	declare -f "cmd_$1" >/dev/null
	ok_or_die "Unknown command: $1. Please use \`$0 help\` for help."

	cmd=cmd_$1
	shift

	# $@ is now a list of command-specific args
	#
	$cmd "$@"
}

main "$@"