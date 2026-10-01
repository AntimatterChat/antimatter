.. _amctl_compliance-export:

amctl compliance-export
-----------------------

Management of compliance exports

Synopsis
~~~~~~~~


Management of compliance exports

Options
~~~~~~~

::

  -h, --help   help for compliance-export

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
* `amctl compliance-export cancel <amctl_compliance-export_cancel.rst>`_ 	 - Cancel compliance export job
* `amctl compliance-export create <amctl_compliance-export_create.rst>`_ 	 - Create a compliance export job, of type 'csv' or 'actiance' or 'globalrelay'
* `amctl compliance-export download <amctl_compliance-export_download.rst>`_ 	 - Download compliance export file
* `amctl compliance-export list <amctl_compliance-export_list.rst>`_ 	 - List compliance export jobs, sorted by creation date descending (newest first)
* `amctl compliance-export show <amctl_compliance-export_show.rst>`_ 	 - Show compliance export job

