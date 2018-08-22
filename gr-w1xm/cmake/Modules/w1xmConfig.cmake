INCLUDE(FindPkgConfig)
PKG_CHECK_MODULES(PC_W1XM w1xm)

FIND_PATH(
    W1XM_INCLUDE_DIRS
    NAMES w1xm/api.h
    HINTS $ENV{W1XM_DIR}/include
        ${PC_W1XM_INCLUDEDIR}
    PATHS ${CMAKE_INSTALL_PREFIX}/include
          /usr/local/include
          /usr/include
)

FIND_LIBRARY(
    W1XM_LIBRARIES
    NAMES gnuradio-w1xm
    HINTS $ENV{W1XM_DIR}/lib
        ${PC_W1XM_LIBDIR}
    PATHS ${CMAKE_INSTALL_PREFIX}/lib
          ${CMAKE_INSTALL_PREFIX}/lib64
          /usr/local/lib
          /usr/local/lib64
          /usr/lib
          /usr/lib64
)

INCLUDE(FindPackageHandleStandardArgs)
FIND_PACKAGE_HANDLE_STANDARD_ARGS(W1XM DEFAULT_MSG W1XM_LIBRARIES W1XM_INCLUDE_DIRS)
MARK_AS_ADVANCED(W1XM_LIBRARIES W1XM_INCLUDE_DIRS)

