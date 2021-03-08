#!/usr/bin/env bats
#
# Tests with gpg signing
#

load helpers

function setup() {
    standard_setup

    # Create dummy gpg keys
    export GNUPGHOME=$TESTDIR/skopeo-gpg
    mkdir --mode=0700 $GNUPGHOME

    # gpg on f30 needs this, otherwise:
    #   gpg: agent_genkey failed: Inappropriate ioctl for device
    # ...but gpg on f29 (and, probably, Ubuntu) doesn't grok this
    GPGOPTS='--pinentry-mode loopback'
    if gpg --pinentry-mode asdf 2>&1 | grep -qi 'Invalid option'; then
        GPGOPTS=
    fi

    for k in alice bob;do
        gpg --batch $GPGOPTS --gen-key --passphrase '' <<END_GPG
Key-Type: RSA
Name-Real: Test key - $k
Name-email: $k@test.redhat.com
%commit
END_GPG

        gpg --armor --export $k@test.redhat.com >$GNUPGHOME/pubkey-$k.gpg
    done

    # Registries. The important part here seems to be sigstore,
    # because (I guess?) the registry itself has no mechanism
    # for storing or validating signatures.
    REGISTRIES_D=$TESTDIR/registries.d
    mkdir $REGISTRIES_D $TESTDIR/sigstore
    cat >$REGISTRIES_D/registries.yaml <<EOF
docker:
   localhost:5000:
        sigstore: file://$TESTDIR/sigstore
EOF

    # Policy file. Basically, require /myns/alice and /myns/bob
    # to be signed; allow /open; and reject anything else.
    POLICY_JSON=$TESTDIR/policy.json
    cat >$POLICY_JSON <<END_POLICY_JSON
{
    "default": [
        {
            "type": "reject"
        }
    ],
    "transports": {
        "docker": {
            "localhost:5000/myns/alice": [
                {
                    "type": "signedBy",
                    "keyType": "GPGKeys",
                    "keyPath": "$GNUPGHOME/pubkey-alice.gpg"
                }
            ],
            "localhost:5000/myns/bob": [
                {
                    "type": "signedBy",
                    "keyType": "GPGKeys",
                    "keyPath": "$GNUPGHOME/pubkey-bob.gpg"
                }
            ],
            "localhost:5000/open": [
                {
                    "type": "insecureAcceptAnything"
                }
            ]
        }
    }
}
END_POLICY_JSON

    start_registry reg
}

@test "signing" {
    run_skopeo '?' standalone-sign /dev/null busybox alice@test.redhat.com -o /dev/null
    if [[ "$output" =~ 'signing is not supported' ]]; then
        skip "skopeo built without support for creating signatures"
        return 1
    fi
    if [ "$status" -ne 0 ]; then
        die "exit code is $status; expected $expected_rc"
    fi

    # Cache local copy
    run_skopeo copy docker://quay.io/libpod/busybox:latest \
               dir:$TESTDIR/busybox

    # Push a bunch of images. Do so *without* --policy flag; this lets us
    # sign or not, creating images that will or won't conform to policy.
    while IFS=$' \t\n' read path sig comments; do
        local sign_opt=
        if [[ $sig != '-' ]]; then
            sign_opt="--sign-by=${sig}@test.redhat.com"
        fi
        run_skopeo --registries.d $REGISTRIES_D \
                   copy --dest-tls-verify=false \
                   $sign_opt \
                   dir:$TESTDIR/busybox \
                   docker://localhost:5000$path
    done <<END_PUSH
/myns/alice:signed        alice    # Properly-signed image
/myns/alice:unsigned      -        # Unsigned image to path that requires signature
/myns/bob:signedbyalice   alice    # Bad signature: image under /bob
/myns/carol:latest        -        # No signature
/open/forall:latest       -        # No signature, but none needed
END_PUSH

    # Done pushing. Now try to fetch. From here on we use the --policy option.
    # The table below lists the paths to fetch, and the expected errors (or
    # none, if we expect them to pass).
    while IFS=$' \t\n' read path expected_error; do
        expected_rc=
        if [[ -n $expected_error ]]; then
            expected_rc=1
        fi

        rm -rf $TESTDIR/d
        run_skopeo $expected_rc \
                   --registries.d $REGISTRIES_D \
                   --policy $POLICY_JSON \
                   copy --src-tls-verify=false \
                   docker://localhost:5000$path \
                   dir:$TESTDIR/d
        if [[ -n $expected_error ]]; then
            expect_output --substring "Source image rejected: $expected_error"
        fi
    done <<END_TESTS
/myns/alice:signed
/myns/bob:signedbyalice    Invalid GPG signature
/myns/alice:unsigned       Signature for identity localhost:5000/myns/alice:signed is not accepted
/myns/carol:latest         Running image docker://localhost:5000/myns/carol:latest is rejected by policy.
/open/forall:latest
END_TESTS
}

teardown() {
    podman rm -f reg

    standard_teardown
}

# vim: filetype=sh
