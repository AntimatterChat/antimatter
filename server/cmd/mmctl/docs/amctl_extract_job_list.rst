.. _amctl_extract_job_list:

amctl extract job list
----------------------

List content extraction jobs

Synopsis
~~~~~~~~


List content extraction jobs

::

  amctl extract job list [flags]

Examples
~~~~~~~~

::

    extract job list

Options
~~~~~~~

::

      --all            Fetch all extract jobs. --page flag will be ignore if provided
  -h, --help           help for list
      --page int       Page number to fetch for the list of extract jobs
      --per-page int   Number of extract jobs to be fetched (default 200)

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

* `amctl extract job <amctl_extract_job.rst>`_ 	 - List and show content extraction jobs

