extern crate clap;
use colored::*;
use clap::{App, Arg};
use std::env;
use std::process::{Command, Output};
use std::io::{self, Write};


fn main() {
    let matches = App::new("Suppr")
        .version("0.0.0")
        .author("John Kwiatkoski")
        .about("Debugger for Kubernetes!")
        .arg(
            Arg::with_name("kubeconfig")
                .short("k")
                .long("kubeconfig")
                .value_name("FILE")
                .help("Specify a kubeconfig file to use")
                .takes_value(true),
        )
        .get_matches();

    let verbose = true;
    // Gets a value for config if supplied by user, or defaults to "default.conf"
    let mut kubeconfig = String::from(matches.value_of("kubeconfig").unwrap_or(""));
    if kubeconfig == "" {
        let home = env::var("HOME");
        if home.is_err() {
            panic!("No kubeconfig provided and HOME environment variable not set");
        } else {
            kubeconfig = format!("{}/{}", home.unwrap(), ".kube/config");
        }
    }
    if verbose {
        println!("Using kubeconfig: {}", kubeconfig);
    }
    // Generate a report
    let mut report = String::from("Kubernetes Diagnostic\n");
    
    // check connectivity
    let api_conn = check_connectivity(&kubeconfig);
    // No need to check output here just success/failure
    report.push_str(&format!("Master connectivity check: {}\n", colorize(api_conn.status.success())));
    
    let node_health = node_health(&kubeconfig);
    // Inversing the result as we check for "NotReady", so a failure
    // is a positive result.
    report.push_str(&format!("Node health check: {}\n", colorize(!node_health.status.success())));
    if verbose || node_health.status.success() {
        report.push_str(&format!("\n{}\n", String::from_utf8(node_health.stdout).expect("Found invalid UTF-8")));
    }
    // check events
    let events = events(&kubeconfig);
    report.push_str(&format!("Events: {}\n", colorize(true) ));
    report.push_str(&format!("Events: {}\n", String::from_utf8(events.stdout).expect("Events output invalid UTF-8")));

    // check pod restarts in kube system
    let pod_restarts = pod_restarts(&kubeconfig);
    report.push_str(&format!("Pods: {}\n", colorize(true) ));
    report.push_str(&format!("{}\n", String::from_utf8(pod_restarts.stdout).expect("Events output invalid UTF-8")));


    println!("{}", report);
}
fn colorize (result: bool) -> String {
    // Unsure if ✗ or failed is better
    if result {
        return "✓".green().to_string();
    } else {
        return "✘".red().to_string();
    }
}

//
// Kubernetes checks
//

fn check_connectivity(kubeconfig: &str) -> Output {
    let result = Command::new("kubectl")
                          .args(&["--kubeconfig", kubeconfig, "get", "nodes"])
                          .output()
                          .expect("Master connectivity failed");

    return result;

}

fn node_health(kubeconfig: &str) -> Output {
    let result = Command::new("kubectl")
                          .args(&["--kubeconfig", kubeconfig, "get", "nodes", "|", "grep", "NotReady"])
                          .output()
                          .expect("Nodes are unhealthy");

    return result;
}

// Json path cant use || for Error or Warning
// JsonPath can't use 'in' for ["Warning", "Error"]
// | grep is %^&* here for some reason
//
// Just returning all events
fn events(kubeconfig: &str) -> Output {
    let result = Command::new("kubectl")
                            .args(&["--kubeconfig", kubeconfig, "get", "events", "-A" ])
                            .output()
                            .expect("Get events failed");
    //println!("Command: {:?}", result);

    return result;
}
fn pod_restarts(kubeconfig: &str) -> Output {
    let result = Command::new("kubectl")
                            .args(&["--kubeconfig", kubeconfig, "get", "pods", "-A"])
                            .output()
                            .expect("Get pods failed");
    //println!("Command: {:?}", result);

    return result;
}
