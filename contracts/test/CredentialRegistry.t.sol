// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {Test} from "forge-std/Test.sol";
import {CredentialRegistry} from "../src/CredentialRegistry.sol";

contract CredentialRegistryTest is Test {
    CredentialRegistry reg;

    address birthDept = address(0xB1);
    address transportDept = address(0x72);
    address educationDept = address(0xED);
    address randomer = address(0xBEEF);

    // Mirrored from the contract as plain constants on purpose: calling
    // reg.DOC_*() inside a test would consume the pending prank/expectRevert.
    uint8 constant BIRTH = 1;
    uint8 constant LICENCE = 2;
    uint8 constant DEGREE = 3;

    bytes32 constant HASH_A = keccak256("birth-cert-a.pdf");
    bytes32 constant HASH_B = keccak256("birth-cert-a-corrected.pdf");
    bytes32 constant HASH_L = keccak256("licence.pdf");

    function setUp() public {
        reg = new CredentialRegistry();
        reg.grantRole(birthDept, BIRTH);
        reg.grantRole(transportDept, LICENCE);
        reg.grantRole(educationDept, DEGREE);
    }

    // --- the demo centerpiece -------------------------------------------------

    /// Transport Dept holds the driving-licence role only. Asking it to issue a
    /// birth certificate must revert, with the role mismatch spelled out.
    function test_TransportDeptCannotIssueBirthCertificate() public {
        vm.prank(transportDept);
        vm.expectRevert(
            abi.encodeWithSelector(
                CredentialRegistry.NotAuthorizedForDocType.selector,
                transportDept,
                uint8(1), // required: birth certificate
                uint8(2) // held: driving licence
            )
        );
        reg.issue(HASH_A, "BC-2024-0001", BIRTH);

        // And nothing was written.
        (bool found,) = reg.verify(HASH_A);
        assertFalse(found, "no record should exist after a reverted issue");
    }

    function test_AddressWithNoRoleCannotIssueAnything() public {
        vm.prank(randomer);
        vm.expectRevert(
            abi.encodeWithSelector(
                CredentialRegistry.NotAuthorizedForDocType.selector, randomer, uint8(1), uint8(0)
            )
        );
        reg.issue(HASH_A, "BC-2024-0001", BIRTH);
    }

    // --- happy paths ----------------------------------------------------------

    function test_BirthDeptIssuesAndVerifies() public {
        vm.prank(birthDept);
        reg.issue(HASH_A, "BC-2024-0001", BIRTH);

        (bool found, CredentialRegistry.Credential memory c) = reg.verify(HASH_A);
        assertTrue(found);
        assertEq(c.docId, "BC-2024-0001");
        assertEq(c.docType, BIRTH);
        assertEq(c.issuer, birthDept);
        assertEq(uint8(c.status), uint8(CredentialRegistry.Status.VALID));
        assertEq(c.prevHash, bytes32(0));
    }

    function test_UnknownHashIsNotFound() public view {
        (bool found,) = reg.verify(keccak256("a pdf nobody ever issued"));
        assertFalse(found);
    }

    function test_Supersede() public {
        vm.startPrank(birthDept);
        reg.issue(HASH_A, "BC-2024-0001", BIRTH);
        reg.supersede(HASH_A, HASH_B, "BC-2024-0001-R1", BIRTH);
        vm.stopPrank();

        (, CredentialRegistry.Credential memory oldC) = reg.verify(HASH_A);
        assertEq(uint8(oldC.status), uint8(CredentialRegistry.Status.SUPERSEDED));

        (bool found, CredentialRegistry.Credential memory newC) = reg.verify(HASH_B);
        assertTrue(found);
        assertEq(uint8(newC.status), uint8(CredentialRegistry.Status.VALID));
        assertEq(newC.prevHash, HASH_A, "new record chains back to the old one");
    }

    function test_Revoke() public {
        vm.startPrank(birthDept);
        reg.issue(HASH_A, "BC-2024-0001", BIRTH);
        reg.revoke(HASH_A, BIRTH);
        vm.stopPrank();

        (, CredentialRegistry.Credential memory c) = reg.verify(HASH_A);
        assertEq(uint8(c.status), uint8(CredentialRegistry.Status.REVOKED));
    }

    // --- other guards ---------------------------------------------------------

    function test_CannotIssueSameHashTwice() public {
        vm.startPrank(birthDept);
        reg.issue(HASH_A, "BC-2024-0001", BIRTH);
        vm.expectRevert(abi.encodeWithSelector(CredentialRegistry.AlreadyIssued.selector, HASH_A));
        reg.issue(HASH_A, "BC-2024-0002", BIRTH);
        vm.stopPrank();
    }

    function test_OtherDeptCannotRevokeYourDocument() public {
        vm.prank(birthDept);
        reg.issue(HASH_A, "BC-2024-0001", BIRTH);

        // Transport Dept has a real role, just not this document's type.
        vm.prank(transportDept);
        vm.expectRevert(
            abi.encodeWithSelector(
                CredentialRegistry.NotAuthorizedForDocType.selector, transportDept, uint8(1), uint8(2)
            )
        );
        reg.revoke(HASH_A, BIRTH);
    }

    /// Same doc type, wrong department: Education Dept cannot revoke a degree it
    /// did not issue. Catches role-only checks that forget issuer identity.
    function test_SameRoleDifferentDeptCannotRevoke() public {
        address otherUni = address(0xED2);
        reg.grantRole(otherUni, DEGREE);

        vm.prank(educationDept);
        reg.issue(HASH_L, "DEG-2024-0001", DEGREE);

        vm.prank(otherUni);
        vm.expectRevert(abi.encodeWithSelector(CredentialRegistry.NotIssuer.selector, otherUni));
        reg.revoke(HASH_L, DEGREE);
    }

    function test_CannotSupersedeARevokedDocument() public {
        vm.startPrank(birthDept);
        reg.issue(HASH_A, "BC-2024-0001", BIRTH);
        reg.revoke(HASH_A, BIRTH);
        vm.expectRevert(abi.encodeWithSelector(CredentialRegistry.NotValid.selector, HASH_A));
        reg.supersede(HASH_A, HASH_B, "BC-2024-0001-R1", BIRTH);
        vm.stopPrank();
    }

    // --- the docId index ------------------------------------------------------

    /// The distinction the whole verify screen rests on. Both files fail the hash
    /// check, but a known docId means the bytes were altered, while an unknown one
    /// means we never issued the document at all.
    function test_TamperedAndNeverIssuedResolveDifferently() public {
        vm.prank(birthDept);
        reg.issue(HASH_A, "BC-2024-0001", BIRTH);

        // Tampered: the id is on file, the hash is not the one we recorded.
        bytes32 expected = reg.currentHashOf("BC-2024-0001");
        assertEq(expected, HASH_A, "a known id resolves to its hash");
        assertTrue(expected != keccak256("birth-cert-a-with-an-edited-date.pdf"), "tampered bytes differ");

        // Never issued: the id resolves to nothing at all.
        assertEq(reg.currentHashOf("BC-9999-999999"), bytes32(0), "an unknown id resolves to zero");
    }

    function test_CurrentHashOfFollowsSupersedeChain() public {
        vm.startPrank(birthDept);
        reg.issue(HASH_A, "BC-2024-0001", BIRTH);
        assertEq(reg.currentHashOf("BC-2024-0001"), HASH_A);

        reg.supersede(HASH_A, HASH_B, "BC-2024-0001-R1", BIRTH);
        vm.stopPrank();

        // Both the original id and the replacement's id point at the new hash, so a
        // QR printed on the old copy still leads to the current document.
        assertEq(reg.currentHashOf("BC-2024-0001"), HASH_B, "old id repoints to the replacement");
        assertEq(reg.currentHashOf("BC-2024-0001-R1"), HASH_B, "new id resolves too");
    }

    function test_RevokeLeavesTheDocumentFindable() public {
        vm.startPrank(birthDept);
        reg.issue(HASH_A, "BC-2024-0001", BIRTH);
        reg.revoke(HASH_A, BIRTH);
        vm.stopPrank();

        assertEq(reg.currentHashOf("BC-2024-0001"), HASH_A, "a revoked document is still findable");
    }

    function test_CannotReuseADocId() public {
        vm.startPrank(birthDept);
        reg.issue(HASH_A, "BC-2024-0001", BIRTH);
        vm.expectRevert(abi.encodeWithSelector(CredentialRegistry.DocIdInUse.selector, "BC-2024-0001"));
        reg.issue(HASH_B, "BC-2024-0001", BIRTH);
        vm.stopPrank();
    }

    function test_OnlyAdminGrantsRoles() public {
        vm.prank(randomer);
        vm.expectRevert(CredentialRegistry.NotAdmin.selector);
        reg.grantRole(randomer, BIRTH);
    }
}
