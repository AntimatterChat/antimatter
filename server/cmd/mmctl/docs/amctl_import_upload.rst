.. _amctl_import_upload:

amctl import upload
-------------------

Upload import files

Synopsis
~~~~~~~~


Upload import files

::

  amctl import upload [filepath] [flags]

Examples
~~~~~~~~

::

    import upload import_file.zip

Options
~~~~~~~

::

  -h, --help            help for upload
      --resume          Set to true to resume an incomplete import upload.
      --upload string   The ID of the import upload to resume.

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

* `amctl import <amctl_import.rst>`_ 	 - Management of imports

