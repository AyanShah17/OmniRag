import uuid
from datetime import datetime, timezone
from typing import Optional, List, Any
from sqlalchemy import (
    Column,
    String,
    Integer,
    BigInteger,
    Boolean,
    DateTime,
    ForeignKey,
    Text,
    JSON,
    Table,
)
from sqlalchemy.orm import declarative_base, relationship

Base = declarative_base()


def generate_uuid() -> str:
    return str(uuid.uuid4())


def utc_now() -> datetime:
    return datetime.now(timezone.utc)


# Association table for Zero-Cost Chunk Re-linking across Document Versions
version_chunks = Table(
    "version_chunks",
    Base.metadata,
    Column("version_id", String(36), ForeignKey("document_versions.id", ondelete="CASCADE"), primary_key=True),
    Column("chunk_id", String(36), ForeignKey("chunks.id", ondelete="CASCADE"), primary_key=True),
    Column("chunk_order", Integer, default=0),
)


class Tenant(Base):
    __tablename__ = "tenants"

    id = Column(String(36), primary_key=True, default=generate_uuid)
    name = Column(String(255), nullable=False)
    plan = Column(String(50), default="free")
    created_at = Column(DateTime(timezone=True), default=utc_now)

    workspaces = relationship("Workspace", back_populates="tenant", cascade="all, delete-orphan")


class Workspace(Base):
    __tablename__ = "workspaces"

    id = Column(String(36), primary_key=True, default=generate_uuid)
    tenant_id = Column(String(36), ForeignKey("tenants.id", ondelete="CASCADE"), nullable=False)
    name = Column(String(255), nullable=False)
    created_at = Column(DateTime(timezone=True), default=utc_now)

    tenant = relationship("Tenant", back_populates="workspaces")
    connectors = relationship("Connector", back_populates="workspace", cascade="all, delete-orphan")
    documents = relationship("Document", back_populates="workspace", cascade="all, delete-orphan")
    conversations = relationship("Conversation", back_populates="workspace", cascade="all, delete-orphan")


class User(Base):
    __tablename__ = "users"

    id = Column(String(36), primary_key=True, default=generate_uuid)
    tenant_id = Column(String(36), ForeignKey("tenants.id", ondelete="CASCADE"), nullable=False)
    email = Column(String(255), unique=True, nullable=False)
    password_hash = Column(String(255), nullable=False)
    role = Column(String(50), default="member")  # admin, member, viewer
    created_at = Column(DateTime(timezone=True), default=utc_now)


class Connector(Base):
    __tablename__ = "connectors"

    id = Column(String(36), primary_key=True, default=generate_uuid)
    workspace_id = Column(String(36), ForeignKey("workspaces.id", ondelete="CASCADE"), nullable=False)
    type = Column(String(50), nullable=False)  # s3, azure_blob, supabase_storage, confluence, local
    name = Column(String(255), nullable=False)
    config = Column(JSON, default=dict)
    is_active = Column(Boolean, default=True)
    sync_frequency = Column(String(50), default="hourly")  # realtime, hourly, daily, manual
    last_synced_at = Column(DateTime(timezone=True), nullable=True)
    created_at = Column(DateTime(timezone=True), default=utc_now)

    workspace = relationship("Workspace", back_populates="connectors")
    documents = relationship("Document", back_populates="connector", cascade="all, delete-orphan")


class Document(Base):
    __tablename__ = "documents"

    id = Column(String(36), primary_key=True, default=generate_uuid)
    workspace_id = Column(String(36), ForeignKey("workspaces.id", ondelete="CASCADE"), nullable=False)
    connector_id = Column(String(36), ForeignKey("connectors.id", ondelete="CASCADE"), nullable=True)
    external_id = Column(String(512), nullable=False)  # S3 Key, Azure URI, Confluence Page ID
    file_name = Column(String(512), nullable=False)
    file_type = Column(String(50), default="text/plain")
    file_size = Column(BigInteger, default=0)
    current_version_id = Column(String(36), nullable=True)
    status = Column(String(50), default="synced")  # pending, syncing, synced, error
    metadata_json = Column("metadata", JSON, default=dict)
    created_at = Column(DateTime(timezone=True), default=utc_now)
    updated_at = Column(DateTime(timezone=True), default=utc_now, onupdate=utc_now)

    workspace = relationship("Workspace", back_populates="documents")
    connector = relationship("Connector", back_populates="documents")
    versions = relationship("DocumentVersion", back_populates="document", cascade="all, delete-orphan")
    chunks = relationship("Chunk", back_populates="document", cascade="all, delete-orphan")


class DocumentVersion(Base):
    __tablename__ = "document_versions"

    id = Column(String(36), primary_key=True, default=generate_uuid)
    document_id = Column(String(36), ForeignKey("documents.id", ondelete="CASCADE"), nullable=False)
    version_number = Column(Integer, nullable=False, default=1)
    file_hash = Column(String(64), nullable=False)  # SHA-256
    total_chunks = Column(Integer, default=0)
    created_at = Column(DateTime(timezone=True), default=utc_now)

    document = relationship("Document", back_populates="versions")
    chunks = relationship("Chunk", secondary=version_chunks, back_populates="versions")


class Chunk(Base):
    __tablename__ = "chunks"

    id = Column(String(36), primary_key=True, default=generate_uuid)
    document_id = Column(String(36), ForeignKey("documents.id", ondelete="CASCADE"), nullable=False)
    chunk_hash = Column(String(64), nullable=False)  # SHA-256 of text
    chunk_index = Column(Integer, nullable=False, default=0)
    text_content = Column(Text, nullable=False)
    token_count = Column(Integer, default=0)
    metadata_json = Column("metadata", JSON, default=dict)
    is_embedded = Column(Boolean, default=False)
    created_at = Column(DateTime(timezone=True), default=utc_now)

    document = relationship("Document", back_populates="chunks")
    versions = relationship("DocumentVersion", secondary=version_chunks, back_populates="chunks")


class Conversation(Base):
    __tablename__ = "conversations"

    id = Column(String(36), primary_key=True, default=generate_uuid)
    workspace_id = Column(String(36), ForeignKey("workspaces.id", ondelete="CASCADE"), nullable=False)
    user_id = Column(String(36), nullable=True)
    title = Column(String(255), default="New Chat")
    created_at = Column(DateTime(timezone=True), default=utc_now)
    updated_at = Column(DateTime(timezone=True), default=utc_now, onupdate=utc_now)

    workspace = relationship("Workspace", back_populates="conversations")
    messages = relationship("Message", back_populates="conversation", cascade="all, delete-orphan")


class Message(Base):
    __tablename__ = "messages"

    id = Column(String(36), primary_key=True, default=generate_uuid)
    conversation_id = Column(String(36), ForeignKey("conversations.id", ondelete="CASCADE"), nullable=False)
    role = Column(String(20), nullable=False)  # user, assistant, system
    content = Column(Text, nullable=False)
    citations = Column(JSON, default=list)  # Grounded citation cards with page, doc_id, snippet, link
    token_count = Column(Integer, default=0)
    created_at = Column(DateTime(timezone=True), default=utc_now)

    conversation = relationship("Conversation", back_populates="messages")
