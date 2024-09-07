#!/bin/sh

update_ubuntu() {
    echo "Updating Ubuntu-based system..."

    # Replace /etc/apt/apt.conf.d/50unattended-upgrades
    cat > /etc/apt/apt.conf.d/50unattended-upgrades << EOL
Unattended-Upgrade::Allowed-Origins {
    "\${distro_id}:\${distro_codename}";
    "\${distro_id}:\${distro_codename}-kernel";
    "\${distro_id}:\${distro_codename}-security";
    "\${distro_id}ESMApps:\${distro_codename}-apps-security";
    "\${distro_id}ESM:\${distro_codename}-infra-security";
    "\${distro_id}:\${distro_codename}-updates";
    "\${distro_id}:\${distro_codename}-proposed";
    "\${distro_id}:\${distro_codename}-backports";
};
Unattended-Upgrade::DevRelease "auto";
Unattended-Upgrade::Remove-Unused-Dependencies "true";
Unattended-Upgrade::Automatic-Reboot "true";
Unattended-Upgrade::Automatic-Reboot-Time "02:00";
EOL

    # Replace /etc/apt/apt.conf.d/10periodic
    cat > /etc/apt/apt.conf.d/10periodic << EOL
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Download-Upgradeable-Packages "1";
APT::Periodic::AutocleanInterval "7";
APT::Periodic::Unattended-Upgrade "1";
EOL

    # Install and enable unattended-upgrades
    apt install -y unattended-upgrades
    systemctl enable unattended-upgrades
    systemctl start unattended-upgrades

    echo "Ubuntu system updated successfully."
}

update_centos() {
    echo "Updating Yum-based system..."

    # Replace /etc/dnf/automatic.conf
    cat > /etc/dnf/automatic.conf << EOL
[commands]
upgrade_type = default
random_sleep = 0
network_online_timeout = 60
download_updates = yes
apply_updates = yes
reboot = when-needed
reboot_command = "shutdown -r +5 'Rebooting after applying package updates'"
[emitters]
emit_via = motd
[email]
email_from = root@example.com
email_to = root
email_host = localhost
[command]
[command_email]
email_from = root@example.com
email_to = root
[base]
debuglevel = 1
EOL

    # Install and enable dnf-automatic
    dnf install -y dnf-automatic
    systemctl enable --now dnf-automatic.timer

    echo "CentOS-based system updated successfully."
}

enable_autoupdate_system() {
    if [ -f /etc/debian_version ]; then
        update_ubuntu
    elif [ -f /etc/redhat-release ]; then
        update_centos
    else
        echo "Unsupported system. This script works only for Ubuntu-based and CentOS-based systems."
        exit 1
    fi
}

# Main execution
if [ "$(id -u)" -ne 0 ]; then
    echo "This script must be run as root" >&2
    exit 1
fi

enable_autoupdate_system
