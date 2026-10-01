.. _amctl_job_update:

amctl job update
----------------

Update the status of a job

Synopsis
~~~~~~~~


Update the status of a job. The following restrictions are in place:
	- in_progress -> pending
	- in_progress | pending -> cancel_requested
	- cancel_requested -> canceled
	
	Those restriction can be bypassed with --force=true but the only statuses you can go to are: pending, cancel_requested and canceled. This can have unexpected consequences and should be used with caution.

::

  amctl job update [job] [status] [flags]

Examples
~~~~~~~~

::

    job update myJobID pending
  	job update myJobID pending --force true
  	job update myJobID canceled --force true

Options
~~~~~~~

::

      --force   Setting a job status is restricted to certain statuses. You can overwrite these restrictions by using --force. This might cause unexpected behaviour on your Antimatter server. Use this option with caution.
  -h, --help    help for update

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

* `amctl job <amctl_job.rst>`_ 	 - Management of jobs

