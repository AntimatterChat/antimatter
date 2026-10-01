.. _amctl_export_download:

amctl export download
---------------------

Download export files

Synopsis
~~~~~~~~


Download export files

::

  amctl export download [exportname] [filepath] [flags]

Examples
~~~~~~~~

::

    # you can indicate the name of the export and its destination path
    $ amctl export download samplename sample_export.zip

    # or if you only indicate the name, the path would match it
    $ amctl export download sample_export.zip

Options
~~~~~~~

::

  -h, --help              help for download
      --num-retries int   Number of retries to do to resume a download. (default 5)

Options inherited from parent commands
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

::

      --config string                path to the configuration file (default "$XDG_CONFIG_HOME/amctl/config")
      --disable-pager                disables paged output
      --insecure-sha1-intermediate   allows to use insecure TLS protocols, such as SHA-1
      --insecure-tls-version         allows to use TLS versions 1.0 and 1.1
      --json                         the output format will be in json format
      --local                        allows communicating with the server through a unix socket
      --quiet                        prevent amctl to generate output for the commands
      --strict                       will only run commands if the amctl version matches the server one
      --suppress-warnings            disables printing warning messages

SEE ALSO
~~~~~~~~

* `amctl export <amctl_export.rst>`_ 	 - Management of exports

