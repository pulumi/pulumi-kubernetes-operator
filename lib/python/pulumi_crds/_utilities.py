from pulumi_kubernetes import _utilities


def get_env(*args):
    return _utilities.get_env(*args)


def get_env_bool(*args):
    return _utilities.get_env_bool(*args)


def get_env_int(*args):
    return _utilities.get_env_int(*args)


def get_env_float(*args):
    return _utilities.get_env_float(*args)


def get_version():
    return _utilities.get_version()
