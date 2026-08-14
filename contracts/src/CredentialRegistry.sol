// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @title CredentialRegistry
/// @notice Anchors government-issued document hashes on chain. The PDFs themselves
///         live off-chain; only sha256(pdf bytes) is stored here.
contract CredentialRegistry {
    enum Status {
        VALID,
        SUPERSEDED,
        REVOKED
    }

    // Document types. 0 is reserved for "no role", so a department with
    // issuerRole[dept] == 0 can issue nothing at all.
    uint8 public constant DOC_NONE = 0;
    uint8 public constant DOC_BIRTH_CERTIFICATE = 1;
    uint8 public constant DOC_DRIVING_LICENCE = 2;
    uint8 public constant DOC_DEGREE_CERTIFICATE = 3;

    struct Credential {
        string docId;
        uint8 docType;
        address issuer;
        uint64 timestamp;
        Status status;
        bytes32 prevHash;
    }

    /// @dev docHash => record. issuer == address(0) means "never issued".
    mapping(bytes32 => Credential) private records;

    /// @dev department address => the single doc type it is allowed to issue.
    mapping(address => uint8) public issuerRole;

    address public immutable admin;

    event RoleGranted(address indexed dept, uint8 docType);
    event Issued(bytes32 indexed docHash, string docId, uint8 docType, address indexed issuer);
    event Superseded(bytes32 indexed oldHash, bytes32 indexed newHash);
    event Revoked(bytes32 indexed docHash);

    error NotAdmin();
    error NotAuthorizedForDocType(address caller, uint8 required, uint8 held);
    error AlreadyIssued(bytes32 docHash);
    error UnknownCredential(bytes32 docHash);
    error NotIssuer(address caller);
    error NotValid(bytes32 docHash);
    error InvalidDocType(uint8 docType);

    constructor() {
        admin = msg.sender;
    }

    /// @notice The gate the whole demo hangs on: a department may only touch
    ///         documents of the exact type its role names.
    modifier onlyIssuerOf(uint8 docType) {
        uint8 held = issuerRole[msg.sender];
        if (held == DOC_NONE || held != docType) {
            revert NotAuthorizedForDocType(msg.sender, docType, held);
        }
        _;
    }

    function grantRole(address dept, uint8 docType) external {
        if (msg.sender != admin) revert NotAdmin();
        if (docType == DOC_NONE || docType > DOC_DEGREE_CERTIFICATE) revert InvalidDocType(docType);
        issuerRole[dept] = docType;
        emit RoleGranted(dept, docType);
    }

    function issue(bytes32 docHash, string calldata docId, uint8 docType) external onlyIssuerOf(docType) {
        if (records[docHash].issuer != address(0)) revert AlreadyIssued(docHash);
        records[docHash] = Credential({
            docId: docId,
            docType: docType,
            issuer: msg.sender,
            timestamp: uint64(block.timestamp),
            status: Status.VALID,
            prevHash: bytes32(0)
        });
        emit Issued(docHash, docId, docType, msg.sender);
    }

    /// @notice Replace a document with a corrected version. The old hash is marked
    ///         SUPERSEDED and the new record points back at it via prevHash.
    function supersede(bytes32 oldHash, bytes32 newHash, string calldata docId, uint8 docType)
        external
        onlyIssuerOf(docType)
    {
        Credential storage old = records[oldHash];
        if (old.issuer == address(0)) revert UnknownCredential(oldHash);
        if (old.issuer != msg.sender) revert NotIssuer(msg.sender);
        if (old.status != Status.VALID) revert NotValid(oldHash);
        if (records[newHash].issuer != address(0)) revert AlreadyIssued(newHash);

        old.status = Status.SUPERSEDED;
        records[newHash] = Credential({
            docId: docId,
            docType: docType,
            issuer: msg.sender,
            timestamp: uint64(block.timestamp),
            status: Status.VALID,
            prevHash: oldHash
        });
        emit Superseded(oldHash, newHash);
        emit Issued(newHash, docId, docType, msg.sender);
    }

    function revoke(bytes32 docHash, uint8 docType) external onlyIssuerOf(docType) {
        Credential storage c = records[docHash];
        if (c.issuer == address(0)) revert UnknownCredential(docHash);
        if (c.issuer != msg.sender) revert NotIssuer(msg.sender);
        c.status = Status.REVOKED;
        emit Revoked(docHash);
    }

    /// @notice Read path for verifiers. `found == false` means NOT_FOUND, which for
    ///         a hash computed off a real PDF means the PDF was tampered with.
    function verify(bytes32 docHash) external view returns (bool found, Credential memory cred) {
        cred = records[docHash];
        found = cred.issuer != address(0);
    }
}
