from setuptools import setup, find_packages

required = ['kubernetes ~= 10.0.1', 'PyYAML==5.3.1']

with open("README.md", "r") as fh:
    long_description = fh.read()

setup(
    name='k8s-triliovault-logcollector',
    version='1.0.0',
    author='TrilioData',
    author_email='support@trilio.io',
    url='http://www.trilio.io/',
    license='http://www.trilio.io/',
    description="This is a python module that collects the information mainly yaml configuration and logs "
                "from k8s cluster for debugging k8s-triliovault application",
    long_description=long_description,
    long_description_content_type="text/markdown",
    classifiers=[
        "Programming Language :: Python :: 3.6",
        "License :: OSI Approved :: MIT License",
        "Operating System :: OS Independent",
    ],
    scripts=['log_collector/log_collector.py'],
    packages=find_packages(exclude=['tests']),
    install_requires=required,
    zip_safe=False,
    python_requires='>=3.6',
)
