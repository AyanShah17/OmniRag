import logging
from sqlalchemy.future import select
from app.db.session import engine, async_session_factory
from app.db.models import Base, Tenant, Workspace, User

logger = logging.getLogger("omnirag.db")


async def init_database():
    logger.info("Initializing database tables...")
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)
    logger.info("Database schema initialized successfully.")

    # Seed default tenant and workspace if not present
    async with async_session_factory() as session:
        result = await session.execute(select(Tenant).where(Tenant.id == "tenant_default"))
        tenant = result.scalar_one_or_none()
        if not tenant:
            tenant = Tenant(
                id="tenant_default",
                name="Enterprise Demo Organization",
                plan="enterprise",
            )
            session.add(tenant)

            workspace = Workspace(
                id="ws_default",
                tenant_id="tenant_default",
                name="Main Knowledge Base",
            )
            session.add(workspace)

            user = User(
                id="user_admin",
                tenant_id="tenant_default",
                email="admin@omnirag.ai",
                password_hash="$2b$12$dummyhashforlocalmvpadminuser2026",
                role="admin",
            )
            session.add(user)
            await session.commit()
            logger.info("Seeded default tenant, workspace (ws_default), and admin user.")
