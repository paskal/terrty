#!/bin/sh

# Function to create Zabbix user and group
create_zabbix_user() {
    echo "Creating zabbix user and group"
    groupadd -g 1997 zabbix
    useradd -u 1997 -g zabbix -G docker zabbix
}

# Function to create and configure the set-zabbix-docker-acl service
setup_docker_acl_service() {
    echo "Creating a service 'set-zabbix-docker-acl' which will allow zabbix to read and write docker socket"
    cat <<EOF >/etc/systemd/system/set-zabbix-docker-acl.service
[Unit]
Description=Zabbix docker ACL Hack
Requires=local-fs.target
After=local-fs.target

[Service]
ExecStart=/usr/bin/setfacl -m u:zabbix:rw /var/run/docker.sock

[Install]
WantedBy=multi-user.target
EOF
}

# Function to install necessary packages and start the service
configure_and_start_service() {
    # acl provides setfacl package
    apt-get -y install acl >/dev/null
    # enable the service on startup and start it
    systemctl enable set-zabbix-docker-acl >/dev/null
    systemctl start set-zabbix-docker-acl
    echo "Docker permissions for Zabbix Agent is done"
}

# Main function to run all steps
setup_zabbix_docker() {
    create_zabbix_user
    setup_docker_acl_service
    configure_and_start_service
}

# Check if script is run as root
if [ "$(id -u)" -ne 0 ]; then
    echo "This script must be run as root" >&2
    exit 1
fi

# Run the main setup function
setup_zabbix_docker
