% skopeo-inspect(1)

## NAME
skopeo\-inspect - Return low-level information about _image-name_ in a registry

## SYNOPSIS
**skopeo inspect** [**--raw**] _image-name_

Return low-level information about _image-name_ in a registry

  **--raw** output raw manifest, default is to format in JSON

  _image-name_ name of image to retrieve information about

  **--authfile** _path_

  Path of the authentication file. Default is ${XDG_RUNTIME\_DIR}/containers/auth.json, which is set using `podman login`.
  If the authorization state is not found there, $HOME/.docker/config.json is checked, which is set using `docker login`.

  **--creds** _username[:password]_ for accessing the registry

  **--cert-dir** _path_ Use certificates at _path_ (*.crt, *.cert, *.key) to connect to the registry

  **--tls-verify** _bool-value_ Require HTTPS and verify certificates when talking to container registries (defaults to true)

## EXAMPLES

To review information for the image fedora from the docker.io registry:
```sh
$ skopeo inspect docker://docker.io/fedora
{
    "Name": "docker.io/library/fedora",
    "Digest": "sha256:a97914edb6ba15deb5c5acf87bd6bd5b6b0408c96f48a5cbd450b5b04509bb7d",
    "RepoTags": [
        "20",
        "21",
        "22",
        "23",
        "24",
        "heisenbug",
        "latest",
        "rawhide"
    ],
    "Created": "2016-06-20T19:33:43.220526898Z",
    "DockerVersion": "1.10.3",
    "Labels": {},
    "Architecture": "amd64",
    "Os": "linux",
    "Layers": [
        "sha256:7c91a140e7a1025c3bc3aace4c80c0d9933ac4ee24b8630a6b0b5d8b9ce6b9d4"
    ]
}
```

# SEE ALSO
skopeo(1), podman-login(1), docker-login(1)

## AUTHORS

Antonio Murdaca <runcom@redhat.com>, Miloslav Trmac <mitr@redhat.com>, Jhon Honce <jhonce@redhat.com>

