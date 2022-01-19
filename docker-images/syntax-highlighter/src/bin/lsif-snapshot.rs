use std::fs;

use sg_syntax::{dump_document, lsif_index};

fn main() {
    if let Some(path) = std::env::args().nth(1) {
        let contents = fs::read_to_string(path).unwrap();
        let document = lsif_index("go", &contents).unwrap();

        println!("\n\n{}", dump_document(document, &contents));
        // println!("{}", dump_document())
    } else {
        panic!("Must pass a filepath");
    }
}
