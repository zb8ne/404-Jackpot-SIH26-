// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {Script, console} from "forge-std/Script.sol";
import {CredentialRegistry} from "../src/CredentialRegistry.sol";

/// Deploys the registry against a local Anvil node and seeds the three
/// department roles onto Anvil's prefunded accounts 0, 1 and 2.
///
///   account 0  0xf39F...2266  Birth Dept      -> birth certificate
///   account 1  0x7099...79C8  Transport Dept  -> driving licence
///   account 2  0x3C44...93BC  Education Dept  -> degree certificate
///
/// Account 0 is both the deployer and the admin, so it is the only address
/// that can hand out further roles.
contract Deploy is Script {
    address constant BIRTH_DEPT = 0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266;
    address constant TRANSPORT_DEPT = 0x70997970C51812dc3A010C7d01b50e0d17dc79C8;
    address constant EDUCATION_DEPT = 0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC;

    function run() external {
        uint256 deployerKey = vm.envUint("DEPLOYER_KEY");

        vm.startBroadcast(deployerKey);

        CredentialRegistry reg = new CredentialRegistry();
        reg.grantRole(BIRTH_DEPT, reg.DOC_BIRTH_CERTIFICATE());
        reg.grantRole(TRANSPORT_DEPT, reg.DOC_DRIVING_LICENCE());
        reg.grantRole(EDUCATION_DEPT, reg.DOC_DEGREE_CERTIFICATE());

        vm.stopBroadcast();

        console.log("CredentialRegistry:", address(reg));
        console.log("  birth dept     :", BIRTH_DEPT);
        console.log("  transport dept :", TRANSPORT_DEPT);
        console.log("  education dept :", EDUCATION_DEPT);

        // The backend reads the address from here; keeps `make demo` from having
        // to scrape stdout.
        vm.writeFile("deployment.txt", vm.toString(address(reg)));
    }
}
