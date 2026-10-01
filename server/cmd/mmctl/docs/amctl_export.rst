.. _amctl_export:

amctl export
------------

Management of exports

Synopsis
~~~~~~~~


Management of exports

Options
~~~~~~~

::

  -h, --help   help for export

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

* `amctl <amctl.rst>`_ 	 - Remote client for the Open Source, self-hosted Slack-alternative
* `amctl export create <amctl_export_create.rst>`_ 	 - Create export file
* `amctl export delete <amctl_export_delete.rst>`_ 	 - Delete export file
* `amctl export download <amctl_export_download.rst>`_ 	 - Download export files
* `amctl export generate-presigned-url <amctl_export_generate-presigned-url.rst>`_ 	 - Generate a presigned url for an export file. This is helpful when an export is big and might have trouble downloading from the Antimatter server.
* `amctl export job <amctl_export_job.rst>`_ 	 - List, show and cancel export jobs
* `amctl export list <amctl_export_list.rst>`_ 	 - List export files

